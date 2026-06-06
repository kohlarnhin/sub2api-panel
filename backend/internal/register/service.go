package register

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"go.uber.org/zap"

	"github.com/zhujiangyong/sub2api-panel/backend/internal/config"
)

const (
	emailOTPUnavailableMessage  = "邮箱验证码无法自动获取，请检查当前用户接收验证码邮箱和 freemail 配置"
	emailOTPResendFetchAttempts = 10
	heroSMSStatusPollInterval   = 10 * time.Second
	heroSMSStatusTimeout        = 180 * time.Second
	heroSMSStatusMaxAttempts    = int(heroSMSStatusTimeout / heroSMSStatusPollInterval)
	heroSMSNumberRetryInterval  = 10 * time.Second
	heroSMSCancelDelay          = 180 * time.Second
	heroSMSCancelRetryInterval  = 15 * time.Second
	maxHeroSMSBatchCount        = 100
	statusCreated               = "created"
	statusRunning               = "running"
	statusPhoneCodeSent         = "phone_code_sent"
	statusCodexEmailRequired    = "codex_email_required"
	statusEmailCodeSent         = "email_code_sent"
	statusSuccess               = "success"
	statusFailed                = "failed"
	statusStopped               = "stopped"
	statusRegistrationBlocked   = "registration_blocked"
)

type Service struct {
	repo           *Repository
	heroSMS        *HeroSMSClient
	sub2api        *Sub2APIClient
	email          *EmailClient
	duck           *DuckClient
	logger         *zap.Logger
	sentinelScript string

	mu       sync.RWMutex
	sessions map[string]*sessionState
	batches  map[string]*batchState

	cancelMu     sync.Mutex
	cancelTasks  map[string]*heroSMSCancelTask
	cancelNotify chan struct{}
	cancelOnce   sync.Once

	userMu      sync.RWMutex
	userRuns    map[int64]*userRunState
	loginQueues map[int64]*userLoginQueue
}

type sessionState struct {
	Session
	apiKey         string
	auth           *AuthSession
	oauthState     string
	codeVerifier   string
	emailVerifyURL string
	emailContinue  string
	// userEmailBindActive 标记本会话正处于「绑定已分配 user_email」的 add-email 流程，
	// 用于区别登录邮箱 OTP（两者共用 verifyEmailCodeWithSession）：仅前者在验证码确认
	// 成功后需要立即把 user_email 标记为已用。
	userEmailBindActive bool
	workerCancel        context.CancelFunc
}

type batchState struct {
	Batch
	payload StartRequest
}

type heroSMSCancelTask struct {
	key          string
	apiKey       string
	activationID string
	phone        string
	reason       string
	createdAt    time.Time
	cancelAfter  time.Time
	attempts     int
	lastError    string
}

type userRunState struct {
	UserRun
	cancel context.CancelFunc
}

type userLoginTask struct {
	UserID    int64
	RunID     string
	SessionID string
	AccountID int64
}

type userLoginQueue struct {
	tasks chan userLoginTask
}

func NewService(repo *Repository, cfg *config.Config, logger *zap.Logger, configPath string) *Service {
	return &Service{
		repo:    repo,
		heroSMS: NewHeroSMSClient(),
		sub2api: NewSub2APIClient(cfg.Sub2API.BaseURL, cfg.Sub2API.APIKey),
		email: NewEmailClient(
			cfg.Freemail.WorkerDomain,
			cfg.Freemail.Token,
			cfg.Freemail.OTPMailbox,
			cfg.Freemail.PollAttempts,
			time.Duration(cfg.Freemail.PollIntervalSeconds)*time.Second,
		),
		logger:         logger,
		duck:           NewDuckClient(),
		sentinelScript: scriptPathFromConfig(configPath),
		sessions:       make(map[string]*sessionState),
		batches:        make(map[string]*batchState),
		cancelTasks:    make(map[string]*heroSMSCancelTask),
		cancelNotify:   make(chan struct{}, 1),
		userRuns:       make(map[int64]*userRunState),
		loginQueues:    make(map[int64]*userLoginQueue),
	}
}

func (s *Service) Template() HeroSMSTemplate {
	return DefaultTemplate()
}

func (s *Service) LoginUser(ctx context.Context, username string) (*UserDashboard, error) {
	user, err := s.repo.GetRegisterUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	return s.UserDashboard(ctx, user.ID)
}

func (s *Service) UserDashboard(ctx context.Context, userID int64) (*UserDashboard, error) {
	user, err := s.repo.GetRegisterUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	summary, err := s.repo.UserSummary(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	loginSummary, err := s.repo.UserLoginSummary(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	accounts, err := s.repo.LatestUserAccounts(ctx, user.ID, 30)
	if err != nil {
		return nil, err
	}
	return &UserDashboard{
		User:           *user,
		Summary:        summary,
		LoginSummary:   loginSummary,
		Run:            s.publicUserRun(user.ID),
		LatestAccounts: accounts,
	}, nil
}

func (s *Service) UpdateUser(ctx context.Context, req UserUpdateRequest) (*UserDashboard, error) {
	user, err := s.repo.UpdateRegisterUser(ctx, req.UserID, req.OTPEmail)
	if err != nil {
		return nil, err
	}
	s.updateActiveUserSessionsOTPEmail(user.ID, user.OTPEmail)
	return s.UserDashboard(ctx, user.ID)
}

func (s *Service) UserEmails(ctx context.Context, userID int64, page, pageSize int, search string) (UserEmailListResponse, error) {
	if _, err := s.repo.GetRegisterUserByID(ctx, userID); err != nil {
		return UserEmailListResponse{}, err
	}
	return s.repo.ListUserEmails(ctx, userID, page, pageSize, search)
}

func (s *Service) GenerateUserEmails(ctx context.Context, req UserEmailGenerateRequest) (*UserEmailGenerateResult, error) {
	user, err := s.repo.GetRegisterUserByID(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	count := req.Count
	if count <= 0 {
		count = 1
	}
	if count > 100 {
		return nil, fmt.Errorf("单次创建邮箱数量不能超过 100")
	}
	if !user.IsDuck {
		return nil, fmt.Errorf("当前用户不是 Duck 邮箱用户，请先在 user_email 表中准备未使用邮箱")
	}
	auth := strings.TrimSpace(req.DuckAuthorization)
	if auth == "" {
		return nil, fmt.Errorf("Duck Authorization 不能为空")
	}

	result := &UserEmailGenerateResult{Emails: []string{}, Errors: []string{}}
	for i := 0; i < count; i++ {
		email, raw, err := s.duck.CreateEmail(ctx, auth)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			continue
		}
		inserted, err := s.repo.InsertUserEmail(ctx, user.ID, email, "duck", raw)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			continue
		}
		if inserted {
			result.Created++
			result.Emails = append(result.Emails, email)
		} else {
			result.Errors = append(result.Errors, fmt.Sprintf("邮箱已存在，已跳过: %s", email))
		}
	}
	summary, err := s.repo.UserSummary(ctx, user.ID)
	if err == nil {
		result.Summary = summary
	}
	if result.Created == 0 && len(result.Errors) > 0 {
		return result, fmt.Errorf("Duck 邮箱创建失败: %s", result.Errors[0])
	}
	return result, nil
}

func (s *Service) StartUserRegister(ctx context.Context, req UserRegisterStartRequest) (*UserDashboard, error) {
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("HeroSMS API Key 不能为空")
	}
	count := req.Count
	if count <= 0 {
		count = 1
	}
	if count > maxHeroSMSBatchCount {
		return nil, fmt.Errorf("单次注册数量不能超过 %d", maxHeroSMSBatchCount)
	}
	user, err := s.repo.GetRegisterUserByID(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(user.OTPEmail) == "" {
		return nil, fmt.Errorf("请先为当前用户配置接收验证码邮箱")
	}
	summary, err := s.repo.UserSummary(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if summary.EmailUnused < count {
		return nil, fmt.Errorf("当前用户未使用邮箱不足：需要 %d 个，当前 %d 个", count, summary.EmailUnused)
	}

	runCtx, cancel := context.WithCancel(context.Background())
	now := time.Now()
	run := &userRunState{
		UserRun: UserRun{
			ID:          newID(),
			UserID:      user.ID,
			Username:    user.Username,
			TargetCount: count,
			Status:      statusRunning,
			Step:        fmt.Sprintf("准备注册 %d 个账号", count),
			Logs: []UserRunLog{{
				Time:    now,
				Level:   "info",
				Message: fmt.Sprintf("任务已创建，目标 %d 个账号", count),
			}},
			CreatedAt: now,
			UpdatedAt: now,
		},
		cancel: cancel,
	}

	s.userMu.Lock()
	if existing := s.userRuns[user.ID]; existing != nil && isActiveUserRunStatus(existing.Status) {
		s.userMu.Unlock()
		cancel()
		return nil, fmt.Errorf("当前用户已有运行中的注册任务")
	}
	s.userRuns[user.ID] = run
	s.userMu.Unlock()

	go s.runUserRegister(runCtx, *user, apiKey, run.ID, count)
	return s.UserDashboard(ctx, user.ID)
}

func (s *Service) StopUserRegister(ctx context.Context, userID int64) (*UserDashboard, error) {
	user, err := s.repo.GetRegisterUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	currentSessionID := ""
	s.userMu.Lock()
	run := s.userRuns[user.ID]
	if run != nil && isActiveUserRunStatus(run.Status) {
		run.StopRequested = true
		run.Status = statusRunning
		run.Step = "已请求停止继续取号，当前激活会加入取消队列，已入队账号继续登录收尾"
		run.UpdatedAt = time.Now()
		currentSessionID = run.CurrentSessionID
		if run.cancel != nil {
			run.cancel()
		}
		run.Logs = appendCappedRunLog(run.Logs, UserRunLog{Time: time.Now(), Level: "warn", Message: "用户请求停止注册任务"})
	}
	s.userMu.Unlock()
	if currentSessionID != "" {
		_, _ = s.Stop(currentSessionID)
	}
	return s.UserDashboard(ctx, user.ID)
}

func (s *Service) StartHeroSMS(ctx context.Context, req StartRequest) (*Session, error) {
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("HeroSMS API Key 不能为空")
	}
	session := s.newSession(apiKey, normalizeGroupIDs(req.GroupIDs), "正在准备 HeroSMS 自动注册")
	runCtx, cancel := context.WithCancel(context.Background())
	session.workerCancel = cancel
	go s.runHeroSMSAutoRegister(runCtx, session.ID)
	return s.publicSession(session.ID), nil
}

func (s *Service) StartHeroSMSBatch(ctx context.Context, req StartRequest) (*Batch, error) {
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("HeroSMS API Key 不能为空")
	}
	count := req.Count
	if count <= 0 {
		count = 1
	}
	if count > maxHeroSMSBatchCount {
		return nil, fmt.Errorf("单次批量创建数量不能超过 %d", maxHeroSMSBatchCount)
	}
	batch := &batchState{
		Batch: Batch{
			ID:          newID(),
			TargetCount: count,
			Status:      statusRunning,
			Step:        fmt.Sprintf("准备批量创建 %d 个账号", count),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		payload: StartRequest{
			APIKey:   apiKey,
			Count:    count,
			GroupIDs: normalizeGroupIDs(req.GroupIDs),
		},
	}
	s.mu.Lock()
	s.batches[batch.ID] = batch
	s.mu.Unlock()

	go s.runBatch(context.Background(), batch.ID)
	return s.publicBatch(batch.ID), nil
}

func (s *Service) GetSession(id string) (*Session, error) {
	session := s.publicSession(id)
	if session == nil {
		return nil, fmt.Errorf("手机号注册会话不存在: %s", id)
	}
	return session, nil
}

func (s *Service) GetBatch(id string) (*Batch, error) {
	batch := s.publicBatch(id)
	if batch == nil {
		return nil, fmt.Errorf("手机号批量注册会话不存在: %s", id)
	}
	return batch, nil
}

func (s *Service) Stop(id string) (*Session, error) {
	s.mu.Lock()
	session := s.sessions[id]
	if session == nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("手机号注册会话不存在: %s", id)
	}
	session.StopRequested = true
	session.Status = statusStopped
	session.Step = "已停止获取手机号，可切换后重新开始"
	session.UpdatedAt = time.Now()
	if session.workerCancel != nil {
		session.workerCancel()
	}
	apiKey := session.apiKey
	activationID := session.HeroSMSActivationID
	phone := session.Phone
	snapshot := cloneSession(&session.Session)
	s.mu.Unlock()

	if activationID != "" && apiKey != "" {
		s.queueHeroSMSCancel(apiKey, activationID, phone, "manual_stop", heroSMSCancelDelay)
	}
	return snapshot, nil
}

func (s *Service) StopBatch(id string) (*Batch, error) {
	s.mu.Lock()
	batch := s.batches[id]
	if batch == nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("手机号批量注册会话不存在: %s", id)
	}
	batch.StopRequested = true
	batch.Status = statusStopped
	batch.Step = "批量注册已停止"
	currentSessionID := batch.CurrentSessionID
	batch.UpdatedAt = time.Now()
	s.mu.Unlock()

	if currentSessionID != "" {
		_, _ = s.Stop(currentSessionID)
	}
	return s.publicBatch(id), nil
}

// verifyEmailCodeWithSession 执行自动获取到的邮箱验证码确认并续接 Codex 流程。
func (s *Service) verifyEmailCodeWithSession(ctx context.Context, sessionID, code string) error {
	session := s.getState(sessionID)
	if session == nil {
		return fmt.Errorf("手机号注册会话不存在: %s", sessionID)
	}
	if session.auth == nil {
		return fmt.Errorf("注册会话已失效")
	}
	s.touch(sessionID, statusRunning, "正在确认邮箱验证码", "", nil)
	respData, err := s.authPostJSON(
		ctx,
		session,
		"https://auth.openai.com/api/accounts/email-otp/validate",
		baseHeaders(firstString(session.emailVerifyURL, "https://auth.openai.com/email-verification")),
		map[string]any{"code": code},
	)
	if err != nil {
		s.touch(sessionID, statusEmailCodeSent, "邮箱验证码确认失败", err.Error(), nil)
		return err
	}
	// email-otp/validate 成功即代表已把分配的 user_email 绑定到账号。无论后续 token 交换
	// 是否成功，都立即把该 user_email 标记为已用，避免下次注册重复选到这个已绑定的邮箱。
	if session.userEmailBindActive && session.UserID > 0 && session.UserEmailID > 0 {
		s.markUserEmailBound(ctx, session)
	}
	return s.finishOrWaitCodex(ctx, sessionID, respData)
}

func (s *Service) sendEmailCode(ctx context.Context, sessionID string) error {
	session := s.getState(sessionID)
	if session == nil {
		return fmt.Errorf("手机号注册会话不存在: %s", sessionID)
	}
	if session.Status != statusCodexEmailRequired && session.Status != statusEmailCodeSent {
		return fmt.Errorf("当前状态为 %s，不能发送邮箱验证码", session.Status)
	}
	if session.auth == nil {
		return fmt.Errorf("注册会话已失效")
	}
	otpMailbox := s.sessionOTPMailbox(session)
	email := strings.TrimSpace(session.Email)
	if email == "" {
		if session.UserID <= 0 {
			return fmt.Errorf("用户会话缺少 user_id，无法分配用户邮箱")
		}
		assigned, err := s.assignUnusedUserEmail(ctx, sessionID, session.UserID)
		if err != nil {
			return err
		}
		email = assigned.Email
	}
	// 标记本会话进入「绑定已分配 user_email」流程（区别于登录邮箱 OTP），使验证码确认
	// 成功后能立即把该 user_email 标记为已用。
	if session.UserID > 0 {
		s.mu.Lock()
		if current := s.sessions[sessionID]; current != nil {
			current.userEmailBindActive = true
		}
		s.mu.Unlock()
	}
	emailPageURL := firstString(session.emailContinue, "https://auth.openai.com/add-email")
	if emailPageURL != "" {
		_ = s.authGetDiscardForSession(ctx, session, emailPageURL)
	}
	autoFetch := s.email.ConfiguredForMailbox(otpMailbox)

	for round := 1; ; round++ {
		baselineID := 0
		if autoFetch {
			baselineID = s.email.LatestEmailIDForMailbox(ctx, otpMailbox)
		}
		sendStep := "正在发送邮箱验证码"
		if round > 1 {
			sendStep = fmt.Sprintf("连续 %d 次未获取到邮箱验证码，正在重新发送（第 %d 轮）", emailOTPResendFetchAttempts, round)
		}
		s.touch(sessionID, statusRunning, sendStep, "", nil)
		s.logUserRunForSession(sessionID, "info", sendStep)
		sendData, err := s.authPostJSON(
			ctx,
			session,
			"https://auth.openai.com/api/accounts/add-email/send",
			baseHeaders(emailPageURL),
			map[string]any{"email": email},
		)
		if err != nil {
			s.touch(sessionID, statusCodexEmailRequired, "邮箱验证码发送失败，可重新发送", err.Error(), sendData)
			return err
		}
		verifyURL := continueURL(sendData)
		s.mu.Lock()
		if current := s.sessions[sessionID]; current != nil {
			current.emailVerifyURL = verifyURL
			current.Email = email
			current.UpdatedAt = time.Now()
		}
		s.mu.Unlock()

		if !autoFetch {
			s.touch(sessionID, statusFailed, emailOTPUnavailableMessage, "", sendData)
			return nil
		}

		// 自动从 freemail OTP 邮箱轮询验证码。每 10 次仍未获取到就重新发送，
		// 避免旧验证码、投递丢失或 freemail 短暂解析失败时一直卡住。
		codeStep := "邮箱验证码已发送，正在自动获取验证码"
		if round > 1 {
			codeStep = fmt.Sprintf("邮箱验证码已重新发送，正在自动获取验证码（第 %d 轮）", round)
		}
		s.touch(sessionID, statusRunning, codeStep, "", sendData)
		s.logUserRunForSession(sessionID, "info", codeStep)
		code, err := s.email.FetchVerificationCodeAttemptsForMailbox(ctx, otpMailbox, baselineID, emailOTPResendFetchAttempts, func() bool {
			return s.isSessionStopped(sessionID)
		}, func(attempt int, fetchErr error) {
			step := fmt.Sprintf("邮箱验证码已发送，正在自动获取验证码（第 %d/%d 次）", attempt, emailOTPResendFetchAttempts)
			if round > 1 {
				step = fmt.Sprintf("邮箱验证码已重新发送，正在自动获取验证码（第 %d 轮，第 %d/%d 次）", round, attempt, emailOTPResendFetchAttempts)
			}
			errText := ""
			if fetchErr != nil {
				errText = fetchErr.Error()
				step = fmt.Sprintf("邮箱验证码暂未获取到，继续重试（第 %d/%d 次）", attempt, emailOTPResendFetchAttempts)
				if round > 1 {
					step = fmt.Sprintf("邮箱验证码暂未获取到，继续重试（第 %d 轮，第 %d/%d 次）", round, attempt, emailOTPResendFetchAttempts)
				}
			}
			s.updatePhoneCodeProgress(sessionID, attempt, emailOTPResendFetchAttempts)
			s.touch(sessionID, statusRunning, step, errText, sendData)
			s.logUserRunForSession(sessionID, "info", step)
		})
		if err != nil {
			s.touch(sessionID, statusFailed, "邮箱验证码自动获取失败，请检查当前用户接收验证码邮箱", err.Error(), sendData)
			return nil
		}
		if code == "" {
			if err := ctx.Err(); err != nil {
				s.touch(sessionID, statusFailed, "邮箱验证码自动获取失败，请检查当前用户接收验证码邮箱", err.Error(), sendData)
				return nil
			}
			step := fmt.Sprintf("连续 %d 次未获取到邮箱验证码，准备重新发送", emailOTPResendFetchAttempts)
			s.touch(sessionID, statusRunning, step, "", sendData)
			s.logUserRunForSession(sessionID, "info", step)
			continue
		}
		s.updatePhoneCodeProgress(sessionID, 0, 0)
		if err := s.verifyEmailCodeWithSession(ctx, sessionID, code); err != nil {
			if ctx.Err() == nil && strings.Contains(err.Error(), "wrong_email_otp_code") {
				step := fmt.Sprintf("邮箱验证码无效，准备重新发送（第 %d 轮）", round+1)
				s.touch(sessionID, statusRunning, step, err.Error(), sendData)
				s.logUserRunForSession(sessionID, "warn", step)
				continue
			}
			s.touch(sessionID, statusFailed, "自动确认邮箱验证码失败，已停止当前账号", err.Error(), sendData)
			return nil
		}
		return nil
	}
}

func (s *Service) sendLoginEmailOTP(ctx context.Context, sessionID string, authData map[string]any) error {
	session := s.getState(sessionID)
	if session == nil {
		return fmt.Errorf("手机号注册会话不存在: %s", sessionID)
	}
	otpMailbox := s.sessionOTPMailbox(session)
	emailPageURL := firstString(continueURL(authData), "https://auth.openai.com/email-verification")
	autoFetch := s.email.ConfiguredForMailbox(otpMailbox)
	baselineID := 0
	if autoFetch {
		baselineID = s.email.LatestEmailIDForMailbox(ctx, otpMailbox)
	}
	s.touch(sessionID, statusRunning, "正在发送登录邮箱验证码", "", nil)
	sendURL := firstString(continueURL(authData), "https://auth.openai.com/api/accounts/email-otp/send")
	sendData, err := s.authGetJSON(ctx, session, sendURL, baseHeaders(firstString(emailPageURL, "https://auth.openai.com/log-in/password")))
	if err != nil {
		s.touch(sessionID, statusCodexEmailRequired, "登录邮箱验证码发送失败", err.Error(), sendData)
		return err
	}
	verifyURL := firstString(continueURL(sendData), emailPageURL, "https://auth.openai.com/email-verification")
	s.mu.Lock()
	if current := s.sessions[sessionID]; current != nil {
		current.emailVerifyURL = verifyURL
		current.RawResponse = cloneMap(sendData)
		current.UpdatedAt = time.Now()
	}
	s.mu.Unlock()
	return s.waitLoginEmailOTP(ctx, sessionID, sendData, baselineID)
}

func (s *Service) waitLoginEmailOTP(ctx context.Context, sessionID string, authData map[string]any, baselineID int) error {
	session := s.getState(sessionID)
	if session == nil {
		return fmt.Errorf("手机号注册会话不存在: %s", sessionID)
	}
	verifyURL := firstString(continueURL(authData), session.emailVerifyURL, "https://auth.openai.com/email-verification")
	s.mu.Lock()
	if current := s.sessions[sessionID]; current != nil {
		current.emailVerifyURL = verifyURL
		current.RawResponse = cloneMap(authData)
		current.UpdatedAt = time.Now()
	}
	s.mu.Unlock()
	otpMailbox := s.sessionOTPMailbox(session)
	autoFetch := s.email.ConfiguredForMailbox(otpMailbox)
	if !autoFetch {
		s.touch(sessionID, statusFailed, emailOTPUnavailableMessage, "", authData)
		return nil
	}
	s.touch(sessionID, statusRunning, "登录邮箱验证码已发送，正在自动获取验证码", "", authData)
	code, err := s.email.FetchVerificationCodeAttemptsForMailbox(ctx, otpMailbox, baselineID, emailOTPResendFetchAttempts, func() bool {
		return s.isSessionStopped(sessionID)
	}, func(attempt int, fetchErr error) {
		step := fmt.Sprintf("登录邮箱验证码已发送，正在自动获取验证码（第 %d/%d 次）", attempt, emailOTPResendFetchAttempts)
		errText := ""
		if fetchErr != nil {
			errText = fetchErr.Error()
		}
		s.updateLoginEmailOTPProgress(sessionID, attempt, emailOTPResendFetchAttempts)
		s.touch(sessionID, statusRunning, step, errText, authData)
	})
	if err != nil || code == "" {
		errText := ""
		if err != nil {
			errText = err.Error()
		}
		s.touch(sessionID, statusFailed, "登录邮箱验证码自动获取失败，请检查当前用户接收验证码邮箱", errText, authData)
		return nil
	}
	s.updateLoginEmailOTPProgress(sessionID, 0, 0)
	if err := s.verifyEmailCodeWithSession(ctx, sessionID, code); err != nil {
		s.touch(sessionID, statusFailed, "自动确认登录邮箱验证码失败，已停止当前账号", err.Error(), authData)
		return nil
	}
	return nil
}

func (s *Service) newSession(apiKey string, groupIDs []int64, step string) *sessionState {
	name, birthdate := randomProfile()
	now := time.Now()
	state := &sessionState{
		Session: Session{
			ID:              newID(),
			Password:        randomPassword(12),
			Name:            name,
			Birthdate:       birthdate,
			Status:          statusRunning,
			Step:            step,
			Template:        DefaultTemplate(),
			GroupIDs:        normalizeGroupIDs(groupIDs),
			CreatedAt:       now,
			UpdatedAt:       now,
			HeroSMSAttempts: []HeroSMSAttempt{},
		},
		apiKey: strings.TrimSpace(apiKey),
	}
	s.mu.Lock()
	s.sessions[state.ID] = state
	s.mu.Unlock()
	return state
}

func (s *Service) newUserSession(apiKey string, user RegisterUser, runID, step string) *sessionState {
	state := s.newSession(apiKey, []int64{user.GroupID}, step)
	s.mu.Lock()
	if current := s.sessions[state.ID]; current != nil {
		current.UserID = user.ID
		current.UserName = user.Username
		current.OTPMailbox = strings.TrimSpace(user.OTPEmail)
		current.RunID = runID
		current.GroupIDs = normalizeGroupIDs([]int64{user.GroupID})
		current.UpdatedAt = time.Now()
	}
	s.mu.Unlock()
	return state
}

func (s *Service) runBatch(ctx context.Context, batchID string) {
	for {
		s.mu.RLock()
		batch := s.batches[batchID]
		if batch == nil {
			s.mu.RUnlock()
			return
		}
		success := batch.SuccessCount
		target := batch.TargetCount
		stop := batch.StopRequested
		s.mu.RUnlock()

		if stop {
			s.touchBatch(batchID, statusStopped, fmt.Sprintf("批量创建已停止：%d/%d", success, target), "")
			return
		}
		if success >= target {
			s.touchBatch(batchID, statusSuccess, fmt.Sprintf("批量创建完成：%d/%d", success, target), "")
			return
		}

		s.mu.RLock()
		payload := s.batches[batchID].payload
		s.mu.RUnlock()

		currentIndex := success + 1
		session := s.newSession(payload.APIKey, payload.GroupIDs, fmt.Sprintf("批量注册 %d/%d：正在准备 HeroSMS 自动注册", currentIndex, target))
		s.mu.Lock()
		if b := s.batches[batchID]; b != nil {
			b.CurrentSessionID = session.ID
			b.SessionIDs = append(b.SessionIDs, session.ID)
			b.Step = fmt.Sprintf("正在创建第 %d/%d 个账号", currentIndex, target)
			b.UpdatedAt = time.Now()
		}
		s.mu.Unlock()

		s.runHeroSMSAutoRegister(ctx, session.ID)
		sessionSnapshot := s.publicSession(session.ID)
		if sessionSnapshot == nil {
			return
		}
		if sessionSnapshot.Status == statusSuccess {
			s.mu.Lock()
			if b := s.batches[batchID]; b != nil {
				b.SuccessCount++
				b.Results = append(b.Results, BatchResult{
					SessionID: sessionSnapshot.ID,
					Phone:     sessionSnapshot.Phone,
					Email:     sessionSnapshot.Email,
					Password:  sessionSnapshot.Password,
					Step:      sessionSnapshot.Step,
				})
				b.Step = fmt.Sprintf("已完成 %d/%d，准备下一个账号", b.SuccessCount, b.TargetCount)
				b.UpdatedAt = time.Now()
			}
			s.mu.Unlock()
			continue
		}

		s.mu.Lock()
		if b := s.batches[batchID]; b != nil {
			b.FailedCount++
			b.Failures = append(b.Failures, BatchFailure{
				SessionID: sessionSnapshot.ID,
				Phone:     sessionSnapshot.Phone,
				Email:     sessionSnapshot.Email,
				Step:      sessionSnapshot.Step,
				Error:     firstString(sessionSnapshot.Error, "注册失败"),
			})
			b.Step = fmt.Sprintf("第 %d 个账号失败，继续创建下一个，已成功 %d/%d", currentIndex, b.SuccessCount, b.TargetCount)
			b.UpdatedAt = time.Now()
		}
		s.mu.Unlock()
	}
}

func (s *Service) runUserRegister(ctx context.Context, user RegisterUser, apiKey, runID string, count int) {
	defer s.maybeFinishUserRun(user.ID, runID)
	for i := 1; i <= count; i++ {
		if ctx.Err() != nil || s.isUserRunStopped(user.ID, runID) {
			s.updateUserRun(user.ID, runID, func(run *userRunState) {
				run.Status = statusRunning
				run.StopRequested = true
				run.Step = fmt.Sprintf("已停止继续取号：手机号阶段完成 %d/%d，等待登录队列收尾", run.PhoneSuccessCount, run.TargetCount)
				run.PhoneDone = true
			})
			return
		}

		session := s.newUserSession(apiKey, user, runID, fmt.Sprintf("注册 %d/%d：正在准备 HeroSMS 自动注册", i, count))
		s.updateUserRun(user.ID, runID, func(run *userRunState) {
			run.CurrentSessionID = session.ID
			run.CurrentPhone = ""
			run.CurrentAccountID = 0
			run.PhoneCodeAttempt = 0
			run.PhoneCodeMaxAttempts = 0
			run.Step = fmt.Sprintf("正在处理第 %d/%d 个手机号", i, count)
			run.Logs = appendCappedRunLog(run.Logs, UserRunLog{
				Time:    time.Now(),
				Level:   "info",
				Message: fmt.Sprintf("开始第 %d/%d 个手机号注册", i, count),
			})
		})

		s.runHeroSMSAutoRegister(ctx, session.ID)
		snapshot := s.publicSession(session.ID)
		if snapshot == nil {
			return
		}
		if snapshot.Status == statusSuccess && snapshot.AccountID > 0 {
			if err := s.repo.UpdateUserAccountStatus(context.Background(), snapshot.AccountID, "queued_login", ""); err != nil {
				s.logger.Warn("queue user account login status failed", zap.Int64("account_id", snapshot.AccountID), zap.Error(err))
			}
			s.updateUserRun(user.ID, runID, func(run *userRunState) {
				run.PhoneSuccessCount++
				run.LoginQueuedCount++
				run.CurrentSessionID = ""
				run.CurrentPhone = snapshot.Phone
				run.CurrentAccountID = snapshot.AccountID
				run.PhoneCodeAttempt = 0
				run.PhoneCodeMaxAttempts = 0
				run.Step = fmt.Sprintf("手机号注册完成 %d/%d，账号 #%d 已进入登录队列", run.PhoneSuccessCount, run.TargetCount, snapshot.AccountID)
				run.Logs = appendCappedRunLog(run.Logs, UserRunLog{
					Time:    time.Now(),
					Level:   "ok",
					Message: fmt.Sprintf("手机号 %s 注册成功，账号 #%d 已投递登录队列", snapshot.Phone, snapshot.AccountID),
				})
			})
			s.enqueueUserLogin(userLoginTask{
				UserID:    user.ID,
				RunID:     runID,
				SessionID: snapshot.ID,
				AccountID: snapshot.AccountID,
			})
			continue
		}
		if snapshot.Status == statusStopped || ctx.Err() != nil {
			s.updateUserRun(user.ID, runID, func(run *userRunState) {
				run.Status = statusRunning
				run.StopRequested = true
				run.Step = fmt.Sprintf("已停止继续取号：手机号阶段完成 %d/%d，等待登录队列收尾", run.PhoneSuccessCount, run.TargetCount)
				run.PhoneDone = true
				run.Logs = appendCappedRunLog(run.Logs, UserRunLog{Time: time.Now(), Level: "warn", Message: "手机号注册任务已停止"})
			})
			return
		}
		s.updateUserRun(user.ID, runID, func(run *userRunState) {
			run.PhoneFailureCount++
			run.PhoneCodeAttempt = 0
			run.PhoneCodeMaxAttempts = 0
			run.Step = fmt.Sprintf("第 %d 个手机号失败，继续下一个", i)
			run.Logs = appendCappedRunLog(run.Logs, UserRunLog{
				Time:    time.Now(),
				Level:   "error",
				Message: fmt.Sprintf("第 %d 个手机号失败：%s", i, firstString(snapshot.Error, snapshot.Step, "未知错误")),
			})
		})
	}
	s.updateUserRun(user.ID, runID, func(run *userRunState) {
		run.PhoneDone = true
		run.CurrentSessionID = ""
		run.CurrentPhone = ""
		run.Step = "手机号注册阶段已完成，等待登录队列处理剩余账号"
		run.Logs = appendCappedRunLog(run.Logs, UserRunLog{Time: time.Now(), Level: "info", Message: "手机号注册阶段已完成"})
	})
}

func (s *Service) enqueueUserLogin(task userLoginTask) {
	if task.UserID <= 0 {
		s.failUserLoginTask(task, fmt.Errorf("登录任务缺少 user_id"))
		return
	}
	queue := s.userLoginQueue(task.UserID)
	queue.tasks <- task
}

func (s *Service) userLoginQueue(userID int64) *userLoginQueue {
	s.userMu.Lock()
	defer s.userMu.Unlock()
	queue := s.loginQueues[userID]
	if queue != nil {
		return queue
	}
	queue = &userLoginQueue{tasks: make(chan userLoginTask, 1000)}
	s.loginQueues[userID] = queue
	go s.runUserLoginQueue(userID, queue)
	return queue
}

func (s *Service) runUserLoginQueue(userID int64, queue *userLoginQueue) {
	for task := range queue.tasks {
		s.processUserLoginTask(task)
	}
}

func (s *Service) processUserLoginTask(task userLoginTask) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	user, err := s.repo.GetRegisterUserByID(ctx, task.UserID)
	if err != nil {
		s.failUserLoginTask(task, err)
		return
	}
	s.mu.Lock()
	if current := s.sessions[task.SessionID]; current != nil {
		current.OTPMailbox = strings.TrimSpace(user.OTPEmail)
		current.UpdatedAt = time.Now()
	}
	s.mu.Unlock()
	s.updateUserRun(task.UserID, task.RunID, func(run *userRunState) {
		run.LoginStartedCount++
		run.CurrentLoginAccountID = task.AccountID
		run.LoginEmailCodeAttempt = 0
		run.LoginEmailCodeMax = 0
		run.Step = fmt.Sprintf("登录队列正在处理账号 #%d", task.AccountID)
		run.Logs = appendCappedRunLog(run.Logs, UserRunLog{
			Time:    time.Now(),
			Level:   "info",
			Message: fmt.Sprintf("账号 #%d 开始登录，目标 Sub2API 分组 %d", task.AccountID, user.GroupID),
		})
	})
	_ = s.repo.UpdateUserAccountStatus(ctx, task.AccountID, "login_running", "")

	email, err := s.assignUnusedUserEmail(ctx, task.SessionID, user.ID)
	if err != nil {
		s.failUserLoginTask(task, err)
		return
	}
	if err := s.repo.AttachUserEmailToAccount(ctx, task.AccountID, *email); err != nil {
		s.failUserLoginTask(task, err)
		return
	}
	if err := s.startCodexLogin(ctx, task.SessionID); err != nil {
		s.failUserLoginTask(task, err)
		return
	}
	snapshot := s.publicSession(task.SessionID)
	if snapshot == nil {
		s.failUserLoginTask(task, fmt.Errorf("登录会话不存在"))
		return
	}
	if snapshot.Status != statusSuccess {
		s.failUserLoginTask(task, errors.New(firstString(snapshot.Error, snapshot.Step, "Codex 登录未完成")))
		return
	}
	s.updateUserRun(task.UserID, task.RunID, func(run *userRunState) {
		run.LoginSuccessCount++
		run.CurrentLoginAccountID = 0
		run.LoginEmailCodeAttempt = 0
		run.LoginEmailCodeMax = 0
		run.Step = fmt.Sprintf("账号 #%d 登录并上传完成", task.AccountID)
		run.Logs = appendCappedRunLog(run.Logs, UserRunLog{
			Time:    time.Now(),
			Level:   "ok",
			Message: fmt.Sprintf("账号 #%d 已绑定邮箱 %s，并上传到分组 %d", task.AccountID, snapshot.Email, user.GroupID),
		})
	})
	s.maybeFinishUserRun(task.UserID, task.RunID)
}

func (s *Service) failUserLoginTask(task userLoginTask, err error) {
	errText := "登录失败"
	if err != nil {
		errText = err.Error()
	}
	_ = s.repo.UpdateUserAccountStatus(context.Background(), task.AccountID, statusFailed, errText)
	s.touch(task.SessionID, statusFailed, "登录队列处理失败", errText, nil)
	s.closeAuthSession(task.SessionID)
	s.updateUserRun(task.UserID, task.RunID, func(run *userRunState) {
		run.LoginFailedCount++
		run.CurrentLoginAccountID = 0
		run.LoginEmailCodeAttempt = 0
		run.LoginEmailCodeMax = 0
		run.Step = fmt.Sprintf("账号 #%d 登录失败，继续处理队列", task.AccountID)
		run.Error = errText
		run.Logs = appendCappedRunLog(run.Logs, UserRunLog{
			Time:    time.Now(),
			Level:   "error",
			Message: fmt.Sprintf("账号 #%d 登录失败：%s", task.AccountID, errText),
		})
	})
	s.maybeFinishUserRun(task.UserID, task.RunID)
}

func (s *Service) runHeroSMSAutoRegister(ctx context.Context, sessionID string) {
	session := s.getState(sessionID)
	if session == nil {
		return
	}
	apiKey := session.apiKey
	template := session.Template
	attempt := 0

	for {
		if s.isSessionStopped(sessionID) || ctx.Err() != nil {
			s.touch(sessionID, statusStopped, "已停止获取手机号，可切换后重新开始", "", nil)
			s.closeAuthSession(sessionID)
			return
		}
		attempt++
		phone := ""
		activationID := ""
		activationFinished := false
		tryCtx, cancel := context.WithTimeout(ctx, 7*time.Minute)
		err := func() error {
			s.closeAuthSession(sessionID)
			step := fmt.Sprintf("正在用 HeroSMS 模板 %s 获取手机号（第 %d 次）", template.Name, attempt)
			s.touch(sessionID, statusRunning, step, "", nil)
			s.logUserRunForSession(sessionID, "info", step)
			numberData, err := s.heroSMS.GetNumber(tryCtx, apiKey, template)
			if err != nil {
				return err
			}
			phone = normalizePhone(firstString(stringValue(numberData["phone_number"]), stringValue(numberData["phoneNumber"])))
			activationID = strings.TrimSpace(stringValue(numberData["activationId"]))
			if phone == "" || activationID == "" {
				return fmt.Errorf("HeroSMS 返回缺少手机号或激活 ID: %s", previewJSON(numberData))
			}

			exists, err := s.repo.PhoneExists(tryCtx, phone)
			if err != nil {
				s.logger.Warn("phone exists check failed", zap.Error(err))
			}
			if exists {
				cancelData := s.queueHeroSMSCancel(apiKey, activationID, phone, "phone_exists", heroSMSCancelDelay)
				step := fmt.Sprintf("手机号 %s 已存在，已加入取消队列，继续重新获取手机号", phone)
				s.touch(sessionID, statusRunning, step, "", map[string]any{"number": numberData, "cancel_queue": cancelData})
				s.logUserRunForSession(sessionID, "warn", step)
				return nil
			}

			s.mu.Lock()
			if current := s.sessions[sessionID]; current != nil {
				current.Phone = phone
				current.HeroSMSActivationID = activationID
				current.HeroSMSAttempt = attempt
				current.HeroSMSAttempts = append(current.HeroSMSAttempts, HeroSMSAttempt{
					Attempt:      attempt,
					ActivationID: activationID,
					Phone:        phone,
					Number:       numberData,
				})
				current.RawResponse = cloneMap(numberData)
				current.Step = fmt.Sprintf("已获取手机号 %s，正在发送注册短信", phone)
				current.UpdatedAt = time.Now()
			}
			s.mu.Unlock()
			s.logUserRunForSession(sessionID, "info", fmt.Sprintf("已获取手机号 %s，激活 ID %s，正在发送注册短信", phone, activationID))

			if err := s.beginPhoneRegister(tryCtx, sessionID); err != nil {
				return err
			}
			s.touch(sessionID, statusRunning, "手机号验证码已发送，正在等待 HeroSMS 验证码", "", nil)
			s.logUserRunForSession(sessionID, "info", fmt.Sprintf("手机号 %s 验证码已发送，正在等待 HeroSMS 返回验证码", phone))
			code, err := s.waitHeroSMSCode(tryCtx, sessionID, apiKey, activationID)
			if err != nil {
				return err
			}
			if code == "" {
				cancelData := s.queueHeroSMSCancel(apiKey, activationID, phone, "sms_timeout", 0)
				if s.isSessionStopped(sessionID) {
					s.touch(sessionID, statusStopped, "已停止获取手机号，当前激活已加入取消队列", "", map[string]any{"activation_id": activationID, "phone": phone, "cancel_queue": cancelData})
					s.logUserRunForSession(sessionID, "warn", fmt.Sprintf("已停止，手机号 %s 当前激活已加入取消队列", phone))
					return nil
				}
				s.touch(sessionID, statusRunning, "3分钟未获取到手机号验证码，已加入取消队列并继续用当前模板重新获取手机号", "", map[string]any{
					"activation_id": activationID,
					"phone":         phone,
					"message":       "HeroSMS 3分钟未返回验证码",
					"cancel_queue":  cancelData,
				})
				s.logUserRunForSession(sessionID, "warn", fmt.Sprintf("手机号 %s 等待 3 分钟未收到验证码，已加入取消队列，继续重新获取手机号", phone))
				return nil
			}
			s.touch(sessionID, statusPhoneCodeSent, "已获取手机号验证码，正在自动确认", "", nil)
			s.logUserRunForSession(sessionID, "info", fmt.Sprintf("手机号 %s 已获取验证码，正在自动确认", phone))
			if err := s.verifyPhoneCode(tryCtx, sessionID, code); err != nil {
				return err
			}
			s.completeHeroSMS(apiKey, activationID)
			activationFinished = true
			return nil
		}()
		cancel()
		if err == nil {
			snapshot := s.publicSession(sessionID)
			if snapshot != nil && isTerminalOrWaitingStatus(snapshot.Status) {
				return
			}
			continue
		}

		s.logger.Warn("HeroSMS phone register attempt failed", zap.Int("attempt", attempt), zap.Error(err))
		if activationID != "" && !activationFinished {
			reason := truncate(err.Error(), 120)
			cancelData := s.queueHeroSMSCancel(apiKey, activationID, phone, reason, heroSMSCancelDelay)
			s.touch(sessionID, statusRunning, fmt.Sprintf("手机号 %s 不可用于新注册，已加入取消队列", phone), "", map[string]any{
				"activation_id": activationID,
				"phone":         phone,
				"error":         err.Error(),
				"cancel_queue":  cancelData,
			})
			s.logUserRunForSession(sessionID, "warn", fmt.Sprintf("手机号 %s 不可用于新注册，已加入取消队列：%s", phone, reason))
		}
		if s.isSessionStopped(sessionID) || ctx.Err() != nil {
			s.touch(sessionID, statusStopped, "已停止获取手机号，可切换后重新开始", "", nil)
			s.logUserRunForSession(sessionID, "warn", "已停止获取手机号")
			s.closeAuthSession(sessionID)
			return
		}
		if !isRetryablePhoneAttemptError(err) {
			s.touch(sessionID, statusFailed, "HeroSMS 自动注册失败", err.Error(), nil)
			s.logUserRunForSession(sessionID, "error", fmt.Sprintf("HeroSMS 自动注册失败：%s", err.Error()))
			s.closeAuthSession(sessionID)
			return
		}
		step := fmt.Sprintf("当前手机号不可用或获取失败，%d 秒后继续获取（第 %d 次）", int(heroSMSNumberRetryInterval.Seconds()), attempt+1)
		if IsHeroSMSNoNumbersError(err) {
			step = fmt.Sprintf("当前 HeroSMS 模板暂无号码，%d 秒后继续用该模板获取（第 %d 次）", int(heroSMSNumberRetryInterval.Seconds()), attempt+1)
		}
		s.touch(sessionID, statusRunning, step, "", map[string]any{"error": err.Error(), "attempt": attempt, "template": template})
		s.logUserRunForSession(sessionID, "info", fmt.Sprintf("%s；上次失败原因：%s", step, err.Error()))
		if s.sleepWithStop(ctx, sessionID, heroSMSNumberRetryInterval) {
			s.touch(sessionID, statusStopped, "已停止获取手机号，可切换后重新开始", "", nil)
			s.logUserRunForSession(sessionID, "warn", "已停止获取手机号")
			s.closeAuthSession(sessionID)
			return
		}
	}
}

func (s *Service) beginPhoneRegister(ctx context.Context, sessionID string) error {
	session := s.getState(sessionID)
	if session == nil {
		return fmt.Errorf("手机号注册会话不存在: %s", sessionID)
	}
	phone := normalizePhone(session.Phone)
	if phone == "" {
		return fmt.Errorf("手机号不能为空")
	}
	auth, err := newAuthSession()
	if err != nil {
		return err
	}
	authURLValue, oauthState, err := generateChatGPTOAuthURL()
	if err != nil {
		return err
	}
	auth.OAuthState = oauthState
	auth.AuthURL = authURLValue

	s.mu.Lock()
	if current := s.sessions[sessionID]; current != nil {
		current.auth = auth
		current.oauthState = oauthState
		current.codeVerifier = ""
		current.Phone = phone
		current.UpdatedAt = time.Now()
	}
	s.mu.Unlock()

	s.touch(sessionID, statusRunning, "正在初始化手机号注册", "", nil)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authURLValue, nil)
	if err != nil {
		return err
	}
	applyBrowserHeaders(req.Header)
	resp, err := auth.Client.Do(req)
	if err != nil {
		return err
	}
	_, _ = ioReadAndClose(resp)
	did := cookieValue(auth.Jar, "https://auth.openai.com/", "oai-did")
	if did == "" {
		did = cookieValue(auth.Jar, "https://openai.com/", "oai-did")
	}
	if did == "" {
		return fmt.Errorf("未获取到 oai-did，可能被 Cloudflare 拦截: %d", resp.StatusCode)
	}

	cookieHeader := authCookieHeader(auth.Jar)
	sentinelHeaders, err := mintSentinelHeaders(ctx, s.sentinelScript, did, "authorize_continue", cookieHeader, "https://auth.openai.com/create-account")
	if err != nil {
		return err
	}
	headers := baseHeaders("https://auth.openai.com/create-account")
	copyHeaders(headers, sentinelHeaders)
	signupData, err := s.authPostJSON(ctx, session, "https://auth.openai.com/api/accounts/authorize/continue", headers, map[string]any{
		"username":    map[string]any{"value": phone, "kind": "phone_number"},
		"screen_hint": "signup",
	})
	if err != nil {
		if isPhoneNumberInUseResponse(signupData, err) {
			err = fmt.Errorf("手机号已被使用，请重新获取: %s", previewAny(signupData))
			s.touch(sessionID, statusRunning, "手机号已被使用，正在更换新手机号", err.Error(), signupData)
			return err
		}
		s.touch(sessionID, statusFailed, "提交手机号注册信息失败", err.Error(), signupData)
		return err
	}

	signupContinueURL := strings.TrimSpace(stringValue(signupData["continue_url"]))
	signupPageType := pageType(signupData)
	passwordPageURL := firstString(signupContinueURL, "https://auth.openai.com/create-account/password")
	if signupPageType != "create_account_password" &&
		signupPageType != "signup_password" &&
		signupPageType != "password" &&
		!strings.Contains(passwordPageURL, "/create-account/password") {
		err := fmt.Errorf("手机号注册未进入设置密码步骤: page_type=%s, continue_url=%s", firstString(signupPageType, "N/A"), firstString(signupContinueURL, "N/A"))
		s.touch(sessionID, statusFailed, "手机号注册步骤不匹配", err.Error(), signupData)
		return err
	}
	if signupContinueURL != "" {
		_ = s.authGetDiscard(ctx, auth, signupContinueURL)
	}

	cookieHeader = authCookieHeader(auth.Jar)
	sentinelHeaders, err = mintSentinelHeaders(ctx, s.sentinelScript, did, "authorize_continue", cookieHeader, passwordPageURL)
	if err != nil {
		return err
	}
	headers = baseHeaders(passwordPageURL)
	copyHeaders(headers, sentinelHeaders)
	registerData, err := s.authPostJSON(ctx, session, "https://auth.openai.com/api/accounts/user/register", headers, map[string]any{
		"password": session.Password,
		"username": phone,
	})
	if err != nil {
		if isPhoneNumberInUseResponse(registerData, err) {
			err = fmt.Errorf("手机号已被使用，请重新获取: %s", previewAny(registerData))
			s.touch(sessionID, statusRunning, "手机号已被使用，正在更换新手机号", err.Error(), registerData)
			return err
		}
		s.touch(sessionID, statusFailed, "手机号注册请求失败", err.Error(), registerData)
		return err
	}

	registerContinueURL := strings.TrimSpace(stringValue(registerData["continue_url"]))
	registerPageType := pageType(registerData)
	if registerPageType == "phone_otp_send" || strings.Contains(registerContinueURL, "/phone-otp/send") {
		sendURL := firstString(registerContinueURL, "https://auth.openai.com/api/accounts/phone-otp/send")
		sendReq, err := http.NewRequestWithContext(ctx, http.MethodGet, sendURL, nil)
		if err != nil {
			return err
		}
		sendReq.Header.Set("referer", "https://auth.openai.com/create-account/password")
		sendReq.Header.Set("accept", "application/json")
		applyBrowserHeaders(sendReq.Header)
		sendResp, err := auth.Client.Do(sendReq)
		if err != nil {
			return err
		}
		sendData, sendBody, err := responseJSON(sendResp)
		if err != nil {
			return err
		}
		if sendResp.StatusCode != http.StatusOK && sendResp.StatusCode != http.StatusNoContent {
			if isPhoneNumberInUseResponse(sendData, errors.New(sendBody)) {
				err := fmt.Errorf("手机号已被使用，请重新获取: %s", truncate(sendBody, 300))
				s.touch(sessionID, statusRunning, "手机号已被使用，正在更换新手机号", err.Error(), sendData)
				return err
			}
			err := fmt.Errorf("手机号验证码发送失败: %d - %s", sendResp.StatusCode, truncate(sendBody, 300))
			s.touch(sessionID, statusFailed, "手机号验证码发送失败", err.Error(), sendData)
			return err
		}
		if len(sendData) > 0 {
			registerData = sendData
		} else {
			registerData = map[string]any{"status_code": sendResp.StatusCode}
		}
	} else if registerPageType == "contact_verification" || strings.Contains(registerContinueURL, "/contact-verification") {
		err := fmt.Errorf("该手机号未进入全新注册短信发送步骤，可能已注册或处于异常注册状态，请更换手机号重新开始")
		s.touch(sessionID, statusFailed, "手机号不可用于全新注册", err.Error(), registerData)
		return err
	} else if registerPageType != "phone_otp_verification" && !strings.Contains(registerContinueURL, "/phone-verification") {
		err := fmt.Errorf("手机号注册未进入短信验证码步骤: page_type=%s, continue_url=%s", firstString(registerPageType, "N/A"), firstString(registerContinueURL, "N/A"))
		s.touch(sessionID, statusFailed, "手机号注册步骤不匹配", err.Error(), registerData)
		return err
	}
	s.touch(sessionID, statusPhoneCodeSent, "手机号验证码已发送，正在等待 HeroSMS 验证码", "", registerData)
	return nil
}

func (s *Service) verifyPhoneCode(ctx context.Context, sessionID, code string) error {
	session := s.getState(sessionID)
	if session == nil {
		return fmt.Errorf("手机号注册会话不存在: %s", sessionID)
	}
	if session.Status != statusPhoneCodeSent {
		return fmt.Errorf("当前状态为 %s，不能确认手机号验证码", session.Status)
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("手机号验证码不能为空")
	}
	s.touch(sessionID, statusRunning, "正在确认手机号验证码", "", nil)
	validateData, err := s.authPostJSON(
		ctx,
		session,
		"https://auth.openai.com/api/accounts/phone-otp/validate",
		baseHeaders("https://auth.openai.com/phone-verification"),
		map[string]any{"code": code},
	)
	if err != nil {
		s.touch(sessionID, statusPhoneCodeSent, "手机号验证码确认失败", err.Error(), validateData)
		return err
	}
	nextData := validateData
	if pageType(validateData) == "about_you" || strings.Contains(continueURL(validateData), "/about-you") {
		created, err := s.createAccountProfile(ctx, sessionID, validateData)
		if err != nil {
			return err
		}
		nextData = created
	}
	if !isChatGPTCallback(nextData) {
		s.touch(sessionID, statusRegistrationBlocked, "手机号验证码已确认，但注册流程尚未返回 ChatGPT 回调", "", nextData)
		return nil
	}
	if session.UserID <= 0 {
		err := fmt.Errorf("用户会话缺少 user_id，无法保存用户账号")
		s.touch(sessionID, statusFailed, "用户账号保存失败", err.Error(), nextData)
		return err
	}
	accountID, err := s.repo.InsertUserPhoneAccount(ctx, session.Session)
	if err != nil {
		s.touch(sessionID, statusFailed, "用户账号保存失败", err.Error(), nextData)
		return err
	}
	s.mu.Lock()
	if current := s.sessions[sessionID]; current != nil {
		current.AccountID = accountID
		current.Status = statusSuccess
		current.Step = "手机号注册完成，已保存基础账号，等待登录队列处理"
		current.Error = ""
		current.RawResponse = cloneMap(nextData)
		current.UpdatedAt = time.Now()
	}
	s.mu.Unlock()
	s.setDatabaseResult(sessionID, map[string]any{"ok": true, "stage": "user_phone_registered", "account_id": accountID})
	s.closeAuthSession(sessionID)
	return nil
}

func (s *Service) createAccountProfile(ctx context.Context, sessionID string, phoneValidateData map[string]any) (map[string]any, error) {
	session := s.getState(sessionID)
	if session == nil || session.auth == nil {
		return nil, fmt.Errorf("注册会话已失效")
	}
	if urlValue := continueURL(phoneValidateData); urlValue != "" {
		_ = s.authGetDiscard(ctx, session.auth, urlValue)
	}
	did := cookieValue(session.auth.Jar, "https://auth.openai.com/", "oai-did")
	headers := baseHeaders("https://auth.openai.com/about-you")
	if did != "" {
		sentinelHeaders, err := mintSentinelHeaders(
			ctx,
			s.sentinelScript,
			did,
			createAccountFlow,
			authCookieHeader(session.auth.Jar),
			"https://auth.openai.com/about-you",
		)
		if err == nil {
			copyHeaders(headers, sentinelHeaders)
		} else {
			s.logger.Warn("create account sentinel failed", zap.Error(err))
		}
	}
	s.touch(sessionID, statusRunning, "正在提交名称和年龄", "", nil)
	data, err := s.authPostJSON(ctx, session, "https://auth.openai.com/api/accounts/create_account", headers, map[string]any{
		"name":      session.Name,
		"birthdate": session.Birthdate,
	})
	if err != nil {
		s.touch(sessionID, statusPhoneCodeSent, "提交名称和年龄失败", err.Error(), data)
		return nil, err
	}
	return data, nil
}

func (s *Service) startCodexLogin(ctx context.Context, sessionID string) error {
	session := s.getState(sessionID)
	if session == nil {
		return fmt.Errorf("手机号注册会话不存在: %s", sessionID)
	}
	auth, err := newAuthSession()
	if err != nil {
		return err
	}
	authURLValue, oauthState, verifier, err := generateCodexOAuthURL()
	if err != nil {
		return err
	}
	auth.OAuthState = oauthState
	auth.CodeVerifier = verifier
	auth.AuthURL = authURLValue
	s.mu.Lock()
	if current := s.sessions[sessionID]; current != nil {
		current.auth = auth
		current.oauthState = oauthState
		current.codeVerifier = verifier
		current.emailContinue = ""
		current.emailVerifyURL = ""
		current.UpdatedAt = time.Now()
	}
	s.mu.Unlock()

	s.touch(sessionID, statusRunning, "正在获取 Codex OAuth 会话", "", nil)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authURLValue, nil)
	if err != nil {
		return err
	}
	applyBrowserHeaders(req.Header)
	resp, err := auth.Client.Do(req)
	if err != nil {
		s.touch(sessionID, statusFailed, "Codex 登录失败", err.Error(), nil)
		return err
	}
	_, _ = ioReadAndClose(resp)
	did := cookieValue(auth.Jar, "https://auth.openai.com/", "oai-did")
	if did == "" {
		err := fmt.Errorf("Codex 登录未获取到 oai-did: %d", resp.StatusCode)
		s.touch(sessionID, statusFailed, "Codex 登录失败", err.Error(), nil)
		return err
	}

	s.touch(sessionID, statusRunning, "正在提交手机号登录", "", nil)
	sentinelHeaders, err := mintSentinelHeaders(ctx, s.sentinelScript, did, "authorize_continue", authCookieHeaderForURL(auth.Jar, "https://auth.openai.com/log-in"), "https://auth.openai.com/log-in")
	if err != nil {
		s.touch(sessionID, statusFailed, "Codex 登录失败", err.Error(), nil)
		return err
	}
	headers := baseHeaders("https://auth.openai.com/log-in")
	copyHeaders(headers, sentinelHeaders)
	loginData, err := s.authPostJSON(ctx, session, "https://auth.openai.com/api/accounts/authorize/continue", headers, map[string]any{
		"username":    map[string]any{"value": session.Phone, "kind": "phone_number"},
		"screen_hint": "login",
	})
	if err != nil {
		s.touch(sessionID, statusFailed, "Codex 手机号登录失败", err.Error(), loginData)
		return err
	}
	if isCodexConsent(loginData) || isCodexEmailRequired(loginData) {
		return s.finishOrWaitCodex(ctx, sessionID, loginData)
	}

	loginContinueURL := continueURL(loginData)
	passwordPageURL := firstString(loginContinueURL, "https://auth.openai.com/log-in/password")
	if pageType(loginData) != "login_password" && !strings.Contains(passwordPageURL, "/log-in/password") {
		err := fmt.Errorf("Codex 登录未进入密码步骤: page_type=%s, continue_url=%s", firstString(pageType(loginData), "N/A"), firstString(loginContinueURL, "N/A"))
		s.touch(sessionID, statusFailed, "Codex 登录失败", err.Error(), loginData)
		return err
	}
	if loginContinueURL != "" {
		_ = s.authGetDiscardForSession(ctx, session, loginContinueURL)
	}
	s.touch(sessionID, statusRunning, "正在验证 Codex 登录密码", "", nil)
	sentinelHeaders, err = mintSentinelHeaders(ctx, s.sentinelScript, did, "authorize_continue", authCookieHeaderForURL(auth.Jar, passwordPageURL), passwordPageURL)
	if err != nil {
		s.touch(sessionID, statusFailed, "Codex 登录失败", err.Error(), nil)
		return err
	}
	headers = baseHeaders(passwordPageURL)
	copyHeaders(headers, sentinelHeaders)
	passwordData, err := s.authPostJSON(
		ctx,
		session,
		"https://auth.openai.com/api/accounts/password/verify",
		headers,
		map[string]any{"password": session.Password},
	)
	if err != nil {
		s.touch(sessionID, statusFailed, "Codex 密码验证失败", err.Error(), passwordData)
		return err
	}
	return s.finishOrWaitCodex(ctx, sessionID, passwordData)
}

func (s *Service) finishOrWaitCodex(ctx context.Context, sessionID string, authData map[string]any) error {
	session := s.getState(sessionID)
	if session == nil {
		return fmt.Errorf("手机号注册会话不存在: %s", sessionID)
	}
	if isCodexLoginEmailOTPSend(authData) {
		if err := s.sendLoginEmailOTP(ctx, sessionID, authData); err != nil {
			return nil
		}
		return nil
	}
	if isCodexLoginEmailOTPWaiting(authData) {
		if err := s.waitLoginEmailOTP(ctx, sessionID, authData, 0); err != nil {
			return nil
		}
		return nil
	}
	if isCodexEmailRequired(authData) {
		cont := continueURL(authData)
		s.mu.Lock()
		if current := s.sessions[sessionID]; current != nil {
			current.emailContinue = cont
			current.UpdatedAt = time.Now()
		}
		s.mu.Unlock()
		s.touch(sessionID, statusCodexEmailRequired, "Codex 登录需要绑定邮箱，正在发送邮箱验证码", "", authData)
		if err := s.sendEmailCode(ctx, sessionID); err != nil {
			return nil
		}
		return nil
	}
	if !isCodexConsent(authData) && continueURL(authData) == "" {
		return fmt.Errorf("Codex 登录返回未知步骤: %s", previewJSON(authData))
	}
	tokenData, err := s.completeCodexAuthorization(ctx, sessionID, authData)
	if err != nil {
		s.touch(sessionID, statusFailed, "Codex 授权失败", err.Error(), authData)
		return err
	}
	email := strings.TrimSpace(firstString(stringValue(tokenData["email"]), session.Email))
	if session.UserID > 0 {
		email = strings.TrimSpace(firstString(session.Email, stringValue(tokenData["email"])))
	}
	if email == "" {
		err := fmt.Errorf("Codex Token 未包含邮箱，无法保存用户账号")
		s.touch(sessionID, statusFailed, "Codex Token 缺少邮箱", err.Error(), authData)
		return err
	}
	s.mu.Lock()
	if current := s.sessions[sessionID]; current != nil {
		current.Email = email
		current.UpdatedAt = time.Now()
	}
	s.mu.Unlock()
	groupIDs := session.GroupIDs
	sub2apiJSON := BuildSub2APIPayload(email, tokenData, groupIDs)
	if session.UserID <= 0 || session.AccountID <= 0 {
		err := fmt.Errorf("用户会话缺少 user_id 或 account_id")
		s.touch(sessionID, statusFailed, "用户账号保存失败", err.Error(), authData)
		return err
	}
	if session.UserEmailID <= 0 {
		err := fmt.Errorf("用户会话缺少 user_email_id")
		s.touch(sessionID, statusFailed, "用户邮箱绑定状态异常", err.Error(), authData)
		return err
	}
	userEmail := UserEmail{ID: session.UserEmailID, UserID: session.UserID, Email: email}
	dbResult := s.repo.SaveUserPhoneAccountToken(ctx, session.AccountID, userEmail, tokenData, sub2apiJSON)
	if ok, _ := dbResult["ok"].(bool); !ok {
		errText := firstString(stringValue(dbResult["error"]), "用户账号保存失败")
		_ = s.repo.UpdateUserAccountStatus(ctx, session.AccountID, statusFailed, errText)
		s.setDatabaseResult(sessionID, dbResult)
		s.touch(sessionID, statusFailed, "用户账号保存失败", errText, authData)
		return errors.New(errText)
	}
	s.setDatabaseResult(sessionID, dbResult)
	uploadResult, err := s.sub2api.Upload(ctx, sub2apiJSON)
	if err != nil {
		_ = s.repo.UpdateUserAccountStatus(ctx, session.AccountID, statusFailed, err.Error())
		s.touch(sessionID, statusFailed, "Token 已保存，但上传 Sub2API 失败", err.Error(), authData)
		return err
	}
	dbResult = s.repo.FinalizeUserPhoneAccount(ctx, session.AccountID, userEmail, tokenData, sub2apiJSON, uploadResult)
	if ok, _ := dbResult["ok"].(bool); !ok {
		errText := firstString(stringValue(dbResult["error"]), "用户账号上传结果保存失败")
		_ = s.repo.UpdateUserAccountStatus(ctx, session.AccountID, statusFailed, errText)
		s.setDatabaseResult(sessionID, dbResult)
		s.touch(sessionID, statusFailed, "用户账号上传结果保存失败", errText, authData)
		return errors.New(errText)
	}
	s.setDatabaseResult(sessionID, dbResult)
	s.touchWithSub2API(sessionID, sub2apiJSON)
	s.mu.Lock()
	if current := s.sessions[sessionID]; current != nil {
		current.Sub2APIUploadResult = uploadResult
		current.Status = statusSuccess
		current.Step = "登录完成，已绑定用户邮箱并上传到用户 Sub2API 分组"
		current.Error = ""
		current.RawResponse = cloneMap(authData)
		current.UpdatedAt = time.Now()
	}
	s.mu.Unlock()
	s.closeAuthSession(sessionID)
	return nil
}

func (s *Service) completeCodexAuthorization(ctx context.Context, sessionID string, authData map[string]any) (map[string]any, error) {
	session := s.getState(sessionID)
	if session == nil || session.auth == nil {
		return nil, fmt.Errorf("注册会话已失效")
	}
	s.touch(sessionID, statusRunning, "正在处理 Codex 授权", "", nil)
	cont := continueURL(authData)

	// 对齐参考实现 auto-register 的 _complete_codex_authorization：
	//  1) continue_url 本身就是 localhost 回调时直接使用——这是 app session 落定后服务端
	//     下发的合法回调，其 code 可直接换 token；
	//  2) 否则从 continue_url（为空时回退到 auth.openai.com 首页，绝不回头重跑 authorize
	//     URL）逐跳跟随 Location 取 localhost 回调；
	//  3) 仍拿不到时，强制走 workspace/select（每个账号必经）落定 app session 后再取回调。
	// 不能把 localhost continue_url 丢弃再重跑 authorize URL：那样换来的 code 未经 app
	// session 确认，换 token 会得到 missing_existing_app_session。
	var (
		callbackURL string
		err         error
	)
	if isLocalCallbackURL(cont) {
		callbackURL = cont
	} else {
		callbackURL, err = s.followForCallback(ctx, session, firstString(cont, "https://auth.openai.com/"))
		if err != nil {
			return nil, err
		}
	}
	if callbackURL == "" {
		// 跟随 consent 跳转拿不到 callback 时，需要先选择 workspace（必要时再选
		// organization），由 workspace/select 返回的 continue_url 才会跳到本地 callback。
		callbackURL, err = s.selectWorkspaceForCallback(ctx, session, cont)
		if err != nil {
			return nil, err
		}
	}
	if callbackURL == "" {
		return nil, fmt.Errorf(
			"Codex 授权未获取到 callback URL: page_type=%s, continue_url=%s",
			firstString(pageType(authData), "N/A"),
			firstString(cont, "N/A"),
		)
	}
	tokenData, err := s.exchangeCallbackForToken(ctx, sessionID, callbackURL)
	if err == nil {
		return tokenData, nil
	}
	if strings.Contains(err.Error(), "missing_existing_app_session") {
		if fallbackURL, fallbackErr := s.selectWorkspaceForCallback(ctx, session, cont); fallbackErr == nil && fallbackURL != "" && fallbackURL != callbackURL {
			return s.exchangeCallbackForToken(ctx, sessionID, fallbackURL)
		}
	}
	return nil, err
}

func (s *Service) followForCallback(ctx context.Context, session *sessionState, startURL string) (string, error) {
	if isLocalCallbackURL(startURL) {
		return startURL, nil
	}
	if session == nil || session.auth == nil {
		return "", fmt.Errorf("注册会话已失效")
	}
	currentURL := startURL
	response, err := s.authGetNoRedirectForSession(ctx, session, currentURL)
	if err != nil {
		return "", err
	}
	for i := 0; i < 20; i++ {
		location := response.Header.Get("Location")
		statusCode := response.StatusCode
		// 只用 Location 头判断 callback，不从响应体抓 localhost——对齐参考实现，
		// 避免抓到 consent 页体内未最终确认的 code 导致 missing_existing_app_session。
		// 仍读尽响应体以便连接复用。
		_, _ = ioReadAndClose(response)
		if isLocalCallbackURL(location) {
			return location, nil
		}
		if location == "" || !isRedirect(statusCode) {
			return "", nil
		}
		nextURL := location
		if parsed, err := url.Parse(location); err == nil && !parsed.IsAbs() {
			base, _ := url.Parse(currentURL)
			nextURL = base.ResolveReference(parsed).String()
		}
		currentURL = nextURL
		response, err = s.authGetNoRedirectForSession(ctx, session, currentURL)
		if err != nil {
			return "", err
		}
	}
	return "", nil
}

// selectWorkspaceForCallback 处理账号归属 workspace/组织时的 Codex 授权收尾：
// 读取 oai-client-auth-session cookie 解出可选 workspace，POST 选择后（必要时再选
// organization），跟随返回的 continue_url 拿到本地 callback URL。对齐参考实现
// auto-register 的 _select_workspace_for_callback。
func (s *Service) selectWorkspaceForCallback(ctx context.Context, session *sessionState, referer string) (string, error) {
	if session == nil || session.auth == nil {
		return "", fmt.Errorf("注册会话已失效")
	}
	authCookie := authSessionCookieValue(session.auth.Jar)
	if authCookie == "" {
		return "", nil
	}
	workspaces := decodeAuthSessionWorkspaces(authCookie)
	if len(workspaces) == 0 {
		return "", fmt.Errorf("登录后未获取到 workspace 信息")
	}
	firstWorkspace, _ := workspaces[0].(map[string]any)
	workspaceID := strings.TrimSpace(stringValue(firstWorkspace["id"]))
	if workspaceID == "" {
		return "", fmt.Errorf("无法解析 workspace_id")
	}

	refererURL := firstString(referer, "https://auth.openai.com/")
	selectData, err := s.authPostJSON(
		ctx,
		session,
		"https://auth.openai.com/api/accounts/workspace/select",
		jsonHeaders(refererURL),
		map[string]any{"workspace_id": workspaceID},
	)
	if err != nil {
		return "", err
	}

	if pageType(selectData) == "organization_select" {
		if orgs := pagePayloadOrgList(selectData); len(orgs) > 0 {
			orgData, err := s.authPostJSON(
				ctx,
				session,
				"https://auth.openai.com/api/accounts/organization/select",
				jsonHeaders(refererURL),
				map[string]any{
					"org_id":     stringValue(orgs[0]["id"]),
					"project_id": stringValue(orgs[0]["default_project_id"]),
				},
			)
			if err != nil {
				return "", err
			}
			selectData = orgData
		}
	}

	finalContinueURL := continueURL(selectData)
	if finalContinueURL == "" {
		return "", fmt.Errorf("workspace/select 缺少 continue_url: %s", previewJSON(selectData))
	}
	return s.followForCallback(ctx, session, finalContinueURL)
}

// decodeAuthSessionWorkspaces 解析 oai-client-auth-session cookie 第一段里的 workspaces。
// tls-client 的 cookie jar 有时会把值存成 URL 编码（如 "." 变成 %2E），原样解不出时
// 回退到 QueryUnescape 再解一次。对齐参考实现 _decode_jwt_segment(cookie.split(".")[0])。
func decodeAuthSessionWorkspaces(cookie string) []any {
	for _, candidate := range cookieDecodeCandidates(cookie) {
		authJSON := decodeJWTHeaderPayload(candidate)
		if workspaces, ok := authJSON["workspaces"].([]any); ok && len(workspaces) > 0 {
			return workspaces
		}
	}
	return nil
}

// cookieDecodeCandidates 返回 cookie 值的候选解码形式：原值，以及 URL 解码后的值
// （仅当解码成功且与原值不同时追加），用于兼容 jar 把值存成百分号编码的情况。
func cookieDecodeCandidates(value string) []string {
	candidates := []string{value}
	if unescaped, err := url.QueryUnescape(value); err == nil && unescaped != value {
		candidates = append(candidates, unescaped)
	}
	return candidates
}

func (s *Service) exchangeCallbackForToken(ctx context.Context, sessionID, callbackURL string) (map[string]any, error) {
	session := s.getState(sessionID)
	if session == nil {
		return nil, fmt.Errorf("手机号注册会话不存在: %s", sessionID)
	}
	parsed, err := url.Parse(callbackURL)
	if err != nil {
		return nil, err
	}
	query := parsed.Query()
	code := firstString(firstQuery(query, "code"))
	state := firstString(firstQuery(query, "state"))
	if state != session.oauthState {
		return nil, fmt.Errorf("OAuth state 不匹配")
	}
	tokenResp, err := exchangeToken(ctx, code, session.codeVerifier)
	if err != nil {
		return nil, err
	}
	idToken := stringValue(tokenResp["id_token"])
	claims := decodeJWTPayload(idToken)
	authClaims, _ := claims["https://api.openai.com/auth"].(map[string]any)
	now := time.Now().UTC()
	expiresIn := int64Value(tokenResp["expires_in"])
	return map[string]any{
		"id_token":      idToken,
		"access_token":  stringValue(tokenResp["access_token"]),
		"refresh_token": stringValue(tokenResp["refresh_token"]),
		"account_id":    stringValue(authClaims["chatgpt_account_id"]),
		"last_refresh":  now.Format(time.RFC3339),
		"email":         firstString(stringValue(claims["email"]), session.Email),
		"type":          "codex",
		"plan_type":     defaultPlan,
		"expires_at":    now.Unix() + expiresIn,
		"expired":       now.Add(time.Duration(expiresIn) * time.Second).Format(time.RFC3339),
	}, nil
}

func (s *Service) waitHeroSMSCode(ctx context.Context, sessionID string, apiKey string, activationID string) (string, error) {
	deadline := time.Now().Add(heroSMSStatusTimeout)
	pollIndex := 0
	var lastStatus map[string]any
	for time.Now().Before(deadline) || time.Now().Equal(deadline) {
		if s.isSessionStopped(sessionID) || ctx.Err() != nil {
			s.touch(sessionID, statusRunning, "已请求停止等待 HeroSMS 短信验证码", "", nil)
			return "", nil
		}
		pollIndex++
		remaining := int(time.Until(deadline).Seconds())
		if remaining < 0 {
			remaining = 0
		}
		s.updatePhoneCodeProgress(sessionID, pollIndex, heroSMSStatusMaxAttempts)
		s.touch(sessionID, statusRunning, fmt.Sprintf("等待 HeroSMS 短信验证码（第 %d 次，剩余 %d 秒）", pollIndex, remaining), "", nil)
		statusData, err := s.heroSMS.GetStatus(ctx, apiKey, activationID)
		if err != nil {
			return "", err
		}
		lastStatus = statusData
		s.setLastHeroSMSStatus(sessionID, statusData)
		s.touch(sessionID, statusRunning, "", "", statusData)
		if code := ExtractHeroSMSCode(statusData); code != "" {
			s.updatePhoneCodeProgress(sessionID, 0, 0)
			return code, nil
		}
		if s.sleepWithStop(ctx, sessionID, heroSMSStatusPollInterval) {
			s.touch(sessionID, statusRunning, "已请求停止等待 HeroSMS 短信验证码", "", nil)
			return "", nil
		}
	}
	s.updatePhoneCodeProgress(sessionID, heroSMSStatusMaxAttempts, heroSMSStatusMaxAttempts)
	s.touch(sessionID, statusRunning, "", "", lastStatus)
	return "", nil
}

// markUserEmailBound 在 user_email 已绑定到账号后，立即把它标记为已用（即使后续 token
// 交换失败也照样占用），避免下次注册重复选到该邮箱。失败仅记录告警，不打断主流程。
func (s *Service) markUserEmailBound(ctx context.Context, session *sessionState) {
	if session == nil || session.UserEmailID <= 0 {
		return
	}
	if err := s.repo.MarkUserEmailUsed(ctx, session.UserEmailID, session.AccountID); err != nil {
		s.logger.Warn("绑定后标记 user_email 已使用失败",
			zap.Int64("user_email_id", session.UserEmailID),
			zap.Int64("account_id", session.AccountID),
			zap.Error(err))
		return
	}
	s.logUserRunForSession(session.ID, "info", fmt.Sprintf("邮箱已绑定账号，已标记 user_email 为已用: %s", session.Email))
}

func (s *Service) assignUnusedUserEmail(ctx context.Context, sessionID string, userID int64) (*UserEmail, error) {
	session := s.getState(sessionID)
	if session == nil {
		return nil, fmt.Errorf("手机号注册会话不存在: %s", sessionID)
	}
	if strings.TrimSpace(session.Email) != "" && session.UserEmailID > 0 {
		return &UserEmail{ID: session.UserEmailID, UserID: userID, Email: session.Email}, nil
	}
	excludes := s.reservedUserEmails(userID, sessionID)
	email, err := s.repo.ClaimUnusedUserEmail(ctx, userID, excludes)
	if err != nil {
		return nil, err
	}
	if email == nil {
		return nil, fmt.Errorf("当前用户没有可用的未使用邮箱，请先创建或导入 user_email")
	}
	s.mu.Lock()
	if current := s.sessions[sessionID]; current != nil {
		current.UserEmailID = email.ID
		current.Email = email.Email
		current.Step = fmt.Sprintf("已分配用户邮箱: %s", email.Email)
		current.UpdatedAt = time.Now()
	}
	s.mu.Unlock()
	return email, nil
}

func (s *Service) reservedUserEmails(userID int64, excludeSessionID string) map[string]struct{} {
	out := make(map[string]struct{})
	s.mu.RLock()
	defer s.mu.RUnlock()
	for id, session := range s.sessions {
		if id == excludeSessionID || session.UserID != userID {
			continue
		}
		email := strings.ToLower(strings.TrimSpace(session.Email))
		if email == "" || session.Status == statusFailed || session.Status == statusSuccess {
			continue
		}
		out[email] = struct{}{}
	}
	return out
}

func (s *Service) authPostJSON(ctx context.Context, session *sessionState, endpoint string, headers http.Header, payload map[string]any) (map[string]any, error) {
	if session.auth == nil {
		return nil, fmt.Errorf("注册会话已失效")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(raw)))
	if err != nil {
		return nil, err
	}
	copyHeaders(req.Header, headers)
	resp, err := session.auth.Client.Do(req)
	if err != nil {
		return nil, err
	}
	data, body, err := responseJSON(resp)
	if err != nil {
		return data, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return data, fmt.Errorf("%s: %d - %s", endpoint, resp.StatusCode, truncate(body, 300))
	}
	return data, nil
}

func (s *Service) authGetJSON(ctx context.Context, session *sessionState, endpoint string, headers http.Header) (map[string]any, error) {
	if session.auth == nil {
		return nil, fmt.Errorf("注册会话已失效")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	copyHeaders(req.Header, headers)
	resp, err := session.auth.Client.Do(req)
	if err != nil {
		return nil, err
	}
	data, body, err := responseJSON(resp)
	if err != nil {
		return data, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return data, fmt.Errorf("%s: %d - %s", endpoint, resp.StatusCode, truncate(body, 300))
	}
	return data, nil
}

func (s *Service) authGetDiscard(ctx context.Context, auth *AuthSession, endpoint string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	applyBrowserHeaders(req.Header)
	resp, err := auth.Client.Do(req)
	if err != nil {
		return err
	}
	_, err = ioReadAndClose(resp)
	return err
}

func (s *Service) authGetDiscardForSession(ctx context.Context, session *sessionState, endpoint string) error {
	if session == nil || session.auth == nil {
		return fmt.Errorf("注册会话已失效")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	applyBrowserHeaders(req.Header)
	resp, err := session.auth.Client.Do(req)
	if err != nil {
		return err
	}
	_, err = ioReadAndClose(resp)
	return err
}

func (s *Service) authGetNoRedirect(ctx context.Context, auth *AuthSession, endpoint string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyBrowserHeaders(req.Header)
	auth.Client.SetFollowRedirect(false)
	defer auth.Client.SetFollowRedirect(true)
	return auth.Client.Do(req)
}

func (s *Service) authGetNoRedirectForSession(ctx context.Context, session *sessionState, endpoint string) (*http.Response, error) {
	if session == nil || session.auth == nil {
		return nil, fmt.Errorf("注册会话已失效")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyBrowserHeaders(req.Header)
	session.auth.Client.SetFollowRedirect(false)
	defer session.auth.Client.SetFollowRedirect(true)
	return session.auth.Client.Do(req)
}

func responseJSON(resp *http.Response) (map[string]any, string, error) {
	defer resp.Body.Close()
	bodyRaw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	body := string(bodyRaw)
	data := map[string]any{}
	if strings.TrimSpace(body) != "" {
		if err := json.Unmarshal(bodyRaw, &data); err != nil {
			data = map[string]any{"raw": strings.TrimSpace(body)}
		}
	}
	return data, body, nil
}

func ioReadAndClose(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

func (s *Service) touch(sessionID string, status string, step string, errText string, raw map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.sessions[sessionID]
	if session == nil {
		return
	}
	if status != "" {
		session.Status = status
	}
	if step != "" {
		session.Step = step
	}
	if errText != "" || status == statusRunning || status == statusSuccess || status == statusStopped || status == statusFailed || status == statusCodexEmailRequired {
		session.Error = errText
	}
	if raw != nil {
		session.RawResponse = cloneMap(raw)
	}
	session.UpdatedAt = time.Now()
}

func (s *Service) touchWithSub2API(sessionID string, sub2apiJSON map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session := s.sessions[sessionID]; session != nil {
		session.Sub2APIJSON = cloneMap(sub2apiJSON)
		session.UpdatedAt = time.Now()
	}
}

func (s *Service) setDatabaseResult(sessionID string, result map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session := s.sessions[sessionID]; session != nil {
		session.DatabaseSaveResult = cloneMap(result)
		session.UpdatedAt = time.Now()
	}
}

func (s *Service) touchBatch(batchID, status, step, errText string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if batch := s.batches[batchID]; batch != nil {
		if status != "" {
			batch.Status = status
		}
		if step != "" {
			batch.Step = step
		}
		batch.Error = errText
		batch.UpdatedAt = time.Now()
	}
}

func (s *Service) setLastHeroSMSStatus(sessionID string, status map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.sessions[sessionID]
	if session == nil || len(session.HeroSMSAttempts) == 0 {
		return
	}
	session.HeroSMSAttempts[len(session.HeroSMSAttempts)-1].LastStatus = cloneMap(status)
	session.UpdatedAt = time.Now()
}

func (s *Service) getState(id string) *sessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[id]
}

func (s *Service) publicSession(id string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session := s.sessions[id]
	if session == nil {
		return nil
	}
	return cloneSession(&session.Session)
}

func (s *Service) publicBatch(id string) *Batch {
	s.mu.RLock()
	defer s.mu.RUnlock()
	batch := s.batches[id]
	if batch == nil {
		return nil
	}
	out := batch.Batch
	out.SessionIDs = append([]string(nil), batch.SessionIDs...)
	out.Results = append([]BatchResult(nil), batch.Results...)
	out.Failures = append([]BatchFailure(nil), batch.Failures...)
	if batch.CurrentSessionID != "" {
		if session := s.sessions[batch.CurrentSessionID]; session != nil {
			out.CurrentSession = cloneSession(&session.Session)
		}
	}
	return &out
}

func (s *Service) publicUserRun(userID int64) *UserRun {
	s.userMu.RLock()
	defer s.userMu.RUnlock()
	run := s.userRuns[userID]
	if run == nil {
		return nil
	}
	return cloneUserRun(&run.UserRun)
}

func cloneUserRun(in *UserRun) *UserRun {
	if in == nil {
		return nil
	}
	out := *in
	out.Logs = append([]UserRunLog(nil), in.Logs...)
	return &out
}

func (s *Service) updateActiveUserSessionsOTPEmail(userID int64, otpEmail string) {
	if userID <= 0 {
		return
	}
	otpEmail = strings.TrimSpace(otpEmail)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, session := range s.sessions {
		if session != nil && session.UserID == userID {
			session.OTPMailbox = otpEmail
			session.UpdatedAt = time.Now()
		}
	}
}

func (s *Service) sessionOTPMailbox(session *sessionState) string {
	if session == nil {
		return ""
	}
	return strings.TrimSpace(session.OTPMailbox)
}

func (s *Service) updateUserRun(userID int64, runID string, update func(*userRunState)) {
	s.userMu.Lock()
	defer s.userMu.Unlock()
	run := s.userRuns[userID]
	if run == nil || run.ID != runID {
		return
	}
	update(run)
	run.UpdatedAt = time.Now()
}

func (s *Service) logUserRunForSession(sessionID, level, message string) {
	if strings.TrimSpace(message) == "" {
		return
	}
	session := s.publicSession(sessionID)
	if session == nil || session.UserID <= 0 || session.RunID == "" {
		return
	}
	s.updateUserRun(session.UserID, session.RunID, func(run *userRunState) {
		run.Logs = appendCappedRunLog(run.Logs, UserRunLog{
			Time:    time.Now(),
			Level:   level,
			Message: message,
		})
		if message != "" {
			run.Step = message
		}
	})
}

func (s *Service) updatePhoneCodeProgress(sessionID string, attempt, maxAttempts int) {
	session := s.publicSession(sessionID)
	if session == nil || session.UserID <= 0 || session.RunID == "" {
		return
	}
	s.updateUserRun(session.UserID, session.RunID, func(run *userRunState) {
		run.PhoneCodeAttempt = attempt
		run.PhoneCodeMaxAttempts = maxAttempts
	})
}

func (s *Service) updateLoginEmailOTPProgress(sessionID string, attempt, maxAttempts int) {
	session := s.publicSession(sessionID)
	if session == nil || session.UserID <= 0 || session.RunID == "" {
		return
	}
	s.updateUserRun(session.UserID, session.RunID, func(run *userRunState) {
		run.LoginEmailCodeAttempt = attempt
		run.LoginEmailCodeMax = maxAttempts
	})
}

func (s *Service) isUserRunStopped(userID int64, runID string) bool {
	s.userMu.RLock()
	defer s.userMu.RUnlock()
	run := s.userRuns[userID]
	return run == nil || run.ID != runID || run.StopRequested
}

func (s *Service) maybeFinishUserRun(userID int64, runID string) {
	s.userMu.Lock()
	defer s.userMu.Unlock()
	run := s.userRuns[userID]
	if run == nil || run.ID != runID || !isActiveUserRunStatus(run.Status) {
		return
	}
	if !run.PhoneDone {
		return
	}
	processedLogins := run.LoginSuccessCount + run.LoginFailedCount
	if processedLogins < run.PhoneSuccessCount {
		run.Step = fmt.Sprintf("等待登录队列：%d/%d", processedLogins, run.PhoneSuccessCount)
		run.UpdatedAt = time.Now()
		return
	}
	run.CurrentSessionID = ""
	run.CurrentPhone = ""
	run.CurrentAccountID = 0
	run.CurrentLoginAccountID = 0
	if run.StopRequested {
		run.Status = statusStopped
		run.Step = fmt.Sprintf("任务已停止：成功 %d，失败 %d", run.LoginSuccessCount, run.PhoneFailureCount+run.LoginFailedCount)
	} else if run.LoginSuccessCount >= run.TargetCount {
		run.Status = statusSuccess
		run.Step = fmt.Sprintf("注册完成：成功 %d/%d", run.LoginSuccessCount, run.TargetCount)
	} else if run.LoginSuccessCount > 0 {
		run.Status = statusSuccess
		run.Step = fmt.Sprintf("注册结束：成功 %d，失败 %d", run.LoginSuccessCount, run.PhoneFailureCount+run.LoginFailedCount)
	} else {
		run.Status = statusFailed
		run.Step = "注册结束，没有成功账号"
		if run.Error == "" {
			run.Error = "没有成功账号"
		}
	}
	run.Logs = appendCappedRunLog(run.Logs, UserRunLog{Time: time.Now(), Level: "info", Message: run.Step})
	run.UpdatedAt = time.Now()
}

func appendCappedRunLog(logs []UserRunLog, item UserRunLog) []UserRunLog {
	if item.Time.IsZero() {
		item.Time = time.Now()
	}
	if item.Level == "" {
		item.Level = "info"
	}
	logs = append(logs, item)
	const maxLogs = 300
	if len(logs) > maxLogs {
		return append([]UserRunLog(nil), logs[len(logs)-maxLogs:]...)
	}
	return logs
}

func isActiveUserRunStatus(status string) bool {
	return status == statusRunning || status == statusPhoneCodeSent || status == statusCodexEmailRequired || status == statusEmailCodeSent
}

func cloneSession(in *Session) *Session {
	if in == nil {
		return nil
	}
	out := *in
	out.RawResponse = cloneMap(in.RawResponse)
	out.Sub2APIJSON = cloneMap(in.Sub2APIJSON)
	out.Sub2APIUploadResult = cloneMap(in.Sub2APIUploadResult)
	out.DatabaseSaveResult = cloneMap(in.DatabaseSaveResult)
	out.GroupIDs = cloneInt64s(in.GroupIDs)
	out.HeroSMSAttempts = append([]HeroSMSAttempt(nil), in.HeroSMSAttempts...)
	for i := range out.HeroSMSAttempts {
		out.HeroSMSAttempts[i].Number = cloneMap(out.HeroSMSAttempts[i].Number)
		out.HeroSMSAttempts[i].LastStatus = cloneMap(out.HeroSMSAttempts[i].LastStatus)
	}
	return &out
}

func (s *Service) isSessionStopped(sessionID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session := s.sessions[sessionID]
	return session == nil || session.StopRequested
}

func (s *Service) sleepWithStop(ctx context.Context, sessionID string, duration time.Duration) bool {
	deadline := time.NewTimer(duration)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return true
		case <-deadline.C:
			return s.isSessionStopped(sessionID)
		case <-ticker.C:
			if s.isSessionStopped(sessionID) {
				return true
			}
		}
	}
}

func (s *Service) closeAuthSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session := s.sessions[sessionID]; session != nil {
		session.auth = nil
		session.UpdatedAt = time.Now()
	}
}

func (s *Service) cancelHeroSMS(apiKey, activationID string) map[string]any {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, err := s.heroSMS.SetStatus(ctx, apiKey, activationID, 8)
	if err != nil {
		statusCode := 0
		if heroErr, ok := err.(*HeroSMSError); ok {
			statusCode = heroErr.StatusCode
		}
		s.logger.Warn("HeroSMS cancel failed", zap.String("activation_id", activationID), zap.Error(err))
		return map[string]any{"error": err.Error(), "status_code": statusCode, "activation_id": activationID, "activation_status": 8}
	}
	return data
}

func (s *Service) completeHeroSMS(apiKey, activationID string) {
	s.removeHeroSMSCancel(apiKey, activationID)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := s.heroSMS.SetStatus(ctx, apiKey, activationID, 6); err != nil {
		s.logger.Warn("HeroSMS complete status failed", zap.String("activation_id", activationID), zap.Error(err))
	}
}

func isRetryablePhoneAttemptError(err error) bool {
	text := err.Error()
	markers := []string{
		"HeroSMS 请求失败",
		"HeroSMS 获取手机号失败",
		"HeroSMS 返回缺少手机号或激活 ID",
		"手机号不可用于全新注册",
		"手机号已被使用",
		"phone_number_in_use",
		"Phone number already in use",
		"手机号验证码发送失败",
		"手机号注册未进入短信验证码步骤",
		"手机号注册未进入设置密码步骤",
		"提交手机号注册信息失败",
		"手机号注册请求失败",
	}
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func (s *Service) queueHeroSMSCancel(apiKey, activationID, phone, reason string, delay time.Duration) map[string]any {
	apiKey = strings.TrimSpace(apiKey)
	activationID = strings.TrimSpace(activationID)
	phone = strings.TrimSpace(phone)
	reason = strings.TrimSpace(reason)
	if apiKey == "" || activationID == "" {
		return map[string]any{
			"queued":            false,
			"activation_id":     activationID,
			"phone":             phone,
			"activation_status": 8,
			"error":             "HeroSMS api_key 或激活 ID 为空，无法加入取消队列",
		}
	}
	if delay < 0 {
		delay = 0
	}
	taskKey := heroSMSCancelTaskKey(apiKey, activationID)
	cancelAfter := time.Now().Add(delay)
	createdAt := time.Now()

	s.cancelMu.Lock()
	if existing := s.cancelTasks[taskKey]; existing != nil {
		existing.apiKey = apiKey
		if phone != "" {
			existing.phone = phone
		}
		if reason != "" {
			existing.reason = reason
		}
		if existing.cancelAfter.Before(cancelAfter) {
			existing.cancelAfter = cancelAfter
		}
	} else {
		s.cancelTasks[taskKey] = &heroSMSCancelTask{
			key:          taskKey,
			apiKey:       apiKey,
			activationID: activationID,
			phone:        phone,
			reason:       reason,
			createdAt:    createdAt,
			cancelAfter:  cancelAfter,
		}
	}
	s.cancelMu.Unlock()

	s.cancelOnce.Do(func() {
		go s.runHeroSMSCancelWorker()
	})
	s.notifyHeroSMSCancelWorker()

	s.logger.Info(
		"HeroSMS activation queued for cancel",
		zap.String("activation_id", activationID),
		zap.String("phone", phone),
		zap.String("reason", reason),
		zap.Duration("delay", delay),
	)
	return map[string]any{
		"queued":            true,
		"activation_id":     activationID,
		"phone":             phone,
		"activation_status": 8,
		"cancel_after":      cancelAfter.Format(time.RFC3339),
		"delay_seconds":     int(delay.Seconds()),
		"reason":            reason,
	}
}

func (s *Service) notifyHeroSMSCancelWorker() {
	select {
	case s.cancelNotify <- struct{}{}:
	default:
	}
}

func (s *Service) runHeroSMSCancelWorker() {
	for {
		task, wait := s.nextHeroSMSCancelTask()
		if task == nil {
			<-s.cancelNotify
			continue
		}
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-timer.C:
			case <-s.cancelNotify:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				continue
			}
		}
		task, ok := s.popDueHeroSMSCancelTask(task.key)
		if !ok {
			continue
		}
		result := s.cancelHeroSMS(task.apiKey, task.activationID)
		if errText := strings.TrimSpace(stringValue(result["error"])); errText != "" {
			statusCode := int(int64Value(result["status_code"]))
			if isTerminalHeroSMSCancelStatus(statusCode) {
				s.logger.Warn(
					"HeroSMS queued cancel failed without retry",
					zap.String("activation_id", task.activationID),
					zap.String("phone", task.phone),
					zap.Int("status_code", statusCode),
					zap.String("error", errText),
				)
				continue
			}
			task.attempts++
			task.lastError = errText
			task.cancelAfter = time.Now().Add(heroSMSCancelRetryInterval)
			s.requeueHeroSMSCancelTask(task)
			s.logger.Warn(
				"HeroSMS queued cancel failed, requeued",
				zap.String("activation_id", task.activationID),
				zap.String("phone", task.phone),
				zap.Int("attempts", task.attempts),
				zap.String("error", errText),
			)
			continue
		}
		s.logger.Info(
			"HeroSMS queued cancel done",
			zap.String("activation_id", task.activationID),
			zap.String("phone", task.phone),
			zap.String("reason", task.reason),
		)
	}
}

func (s *Service) nextHeroSMSCancelTask() (*heroSMSCancelTask, time.Duration) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	if len(s.cancelTasks) == 0 {
		return nil, 0
	}
	var chosen *heroSMSCancelTask
	for _, task := range s.cancelTasks {
		if chosen == nil || task.cancelAfter.Before(chosen.cancelAfter) {
			chosen = task
		}
	}
	if chosen == nil {
		return nil, 0
	}
	out := *chosen
	wait := time.Until(out.cancelAfter)
	if wait < 0 {
		wait = 0
	}
	return &out, wait
}

func (s *Service) popDueHeroSMSCancelTask(taskKey string) (*heroSMSCancelTask, bool) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	task := s.cancelTasks[taskKey]
	if task == nil || time.Now().Before(task.cancelAfter) {
		return nil, false
	}
	delete(s.cancelTasks, taskKey)
	out := *task
	return &out, true
}

func (s *Service) requeueHeroSMSCancelTask(task *heroSMSCancelTask) {
	if task == nil {
		return
	}
	s.cancelMu.Lock()
	s.cancelTasks[task.key] = task
	s.cancelMu.Unlock()
	s.notifyHeroSMSCancelWorker()
}

func (s *Service) removeHeroSMSCancel(apiKey, activationID string) {
	apiKey = strings.TrimSpace(apiKey)
	activationID = strings.TrimSpace(activationID)
	if apiKey == "" || activationID == "" {
		return
	}
	taskKey := heroSMSCancelTaskKey(apiKey, activationID)
	s.cancelMu.Lock()
	delete(s.cancelTasks, taskKey)
	s.cancelMu.Unlock()
	s.notifyHeroSMSCancelWorker()
}

func heroSMSCancelTaskKey(apiKey, activationID string) string {
	return strings.TrimSpace(apiKey) + "\x00" + strings.TrimSpace(activationID)
}

func isTerminalHeroSMSCancelStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity:
		return true
	default:
		return false
	}
}

func isPhoneNumberInUseResponse(data map[string]any, err error) bool {
	if errorData, ok := data["error"].(map[string]any); ok {
		code := strings.TrimSpace(stringValue(errorData["code"]))
		message := strings.TrimSpace(stringValue(errorData["message"]))
		if code == "phone_number_in_use" || strings.Contains(message, "Phone number already in use") {
			return true
		}
	}
	if err == nil {
		return false
	}
	text := err.Error()
	return strings.Contains(text, "phone_number_in_use") || strings.Contains(text, "Phone number already in use")
}

func isTerminalOrWaitingStatus(status string) bool {
	return status == statusSuccess ||
		status == statusCodexEmailRequired ||
		status == statusEmailCodeSent ||
		status == statusRegistrationBlocked
}

func pageType(data map[string]any) string {
	page, _ := data["page"].(map[string]any)
	return strings.TrimSpace(stringValue(page["type"]))
}

func continueURL(data map[string]any) string {
	return strings.TrimSpace(stringValue(data["continue_url"]))
}

func isChatGPTCallback(data map[string]any) bool {
	cont := continueURL(data)
	payload := pagePayload(data)
	payloadURL := strings.TrimSpace(stringValue(payload["url"]))
	return strings.Contains(cont, "https://chatgpt.com/api/auth/callback/openai") || strings.Contains(payloadURL, "https://chatgpt.com/api/auth/callback/openai")
}

func isCodexConsent(data map[string]any) bool {
	pt := pageType(data)
	cont := continueURL(data)
	return pt == "sign_in_with_chatgpt_codex_consent" ||
		strings.Contains(cont, "/sign-in-with-chatgpt/codex/consent") ||
		strings.HasPrefix(cont, "http://localhost")
}

func isCodexEmailRequired(data map[string]any) bool {
	pt := pageType(data)
	cont := continueURL(data)
	return pt == "add_email" || strings.Contains(cont, "/add-email")
}

func isCodexLoginEmailOTPSend(data map[string]any) bool {
	pt := pageType(data)
	cont := continueURL(data)
	return pt == "email_otp_send" || strings.Contains(cont, "/email-otp/send")
}

func isCodexLoginEmailOTPWaiting(data map[string]any) bool {
	pt := pageType(data)
	cont := continueURL(data)
	return pt == "email_otp_verification" || pt == "email_verification" || pt == "contact_verification" ||
		strings.Contains(cont, "/email-verification") || strings.Contains(cont, "/contact-verification")
}

func pagePayload(data map[string]any) map[string]any {
	page, _ := data["page"].(map[string]any)
	payload, _ := page["payload"].(map[string]any)
	return payload
}

func pagePayloadOrgList(data map[string]any) []map[string]any {
	payload := pagePayload(data)
	nested, _ := payload["data"].(map[string]any)
	raw, _ := nested["orgs"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, obj)
		}
	}
	return out
}

func jsonHeaders(refs ...string) http.Header {
	h := http.Header{}
	h.Set("accept", "application/json")
	h.Set("content-type", "application/json")
	h.Set("referer", firstString(refs...))
	applyBrowserHeaders(h)
	return h
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Set(key, value)
		}
	}
}

func isRedirect(status int) bool {
	return status == http.StatusMovedPermanently ||
		status == http.StatusFound ||
		status == http.StatusSeeOther ||
		status == http.StatusTemporaryRedirect ||
		status == http.StatusPermanentRedirect
}

func firstQuery(values url.Values, key string) string {
	items := values[key]
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

func decodeJWTHeaderPayload(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) == 0 {
		return map[string]any{}
	}
	raw := parts[0]
	if mod := len(raw) % 4; mod != 0 {
		raw += strings.Repeat("=", 4-mod)
	}
	data, err := base64DecodeURL(raw)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func base64DecodeURL(raw string) ([]byte, error) {
	if strings.Contains(raw, "=") {
		return base64.URLEncoding.DecodeString(raw)
	}
	return base64.RawURLEncoding.DecodeString(raw)
}
