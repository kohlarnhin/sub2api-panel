package register

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) EnsureSchema(ctx context.Context) error {
	statements := []string{
		`
		CREATE TABLE IF NOT EXISTS register_users (
			id BIGSERIAL PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL DEFAULT '5nuEGNrh7h4km5aTAy81',
			group_id BIGINT NOT NULL,
			is_duck BOOLEAN NOT NULL DEFAULT FALSE,
			otp_email TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
		`,
		`ALTER TABLE register_users ADD COLUMN IF NOT EXISTS password TEXT NOT NULL DEFAULT '5nuEGNrh7h4km5aTAy81'`,
		`ALTER TABLE register_users ALTER COLUMN password SET DEFAULT '5nuEGNrh7h4km5aTAy81'`,
		`UPDATE register_users SET password = '5nuEGNrh7h4km5aTAy81' WHERE COALESCE(password, '') = ''`,
		`ALTER TABLE register_users ALTER COLUMN password SET NOT NULL`,
		`ALTER TABLE register_users ADD COLUMN IF NOT EXISTS otp_email TEXT NOT NULL DEFAULT ''`,
		`
		CREATE TABLE IF NOT EXISTS user_email (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES register_users(id) ON DELETE CASCADE,
			email TEXT NOT NULL,
			provider TEXT NOT NULL DEFAULT 'manual',
			raw_response TEXT NOT NULL DEFAULT '{}',
			used_at TIMESTAMPTZ DEFAULT NULL,
			account_id BIGINT DEFAULT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (user_id, email)
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS user_phone_accounts (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES register_users(id) ON DELETE CASCADE,
			user_email_id BIGINT REFERENCES user_email(id) ON DELETE SET NULL,
			phone TEXT NOT NULL,
			email TEXT DEFAULT '',
			password TEXT NOT NULL,
			name TEXT DEFAULT '',
			birthdate TEXT DEFAULT '',
			status TEXT NOT NULL DEFAULT 'registered',
			error TEXT DEFAULT '',
			token TEXT NOT NULL DEFAULT '{}',
			sub2api_json TEXT NOT NULL DEFAULT '{}',
			sub2api_upload_result TEXT NOT NULL DEFAULT '{}',
			phone_session_id TEXT DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS register_user_page_config (
			user_id BIGINT PRIMARY KEY REFERENCES register_users(id) ON DELETE CASCADE,
			herosms_api_key TEXT NOT NULL DEFAULT '',
			duck_authorization TEXT NOT NULL DEFAULT '',
			register_count INT NOT NULL DEFAULT 1,
			email_count INT NOT NULL DEFAULT 1,
			global_proxy TEXT NOT NULL DEFAULT '',
			proxy_sms_enabled BOOLEAN NOT NULL DEFAULT FALSE,
			proxy_openai_enabled BOOLEAN NOT NULL DEFAULT FALSE,
			proxy_email_enabled BOOLEAN NOT NULL DEFAULT FALSE,
			proxy_sub2api_enabled BOOLEAN NOT NULL DEFAULT FALSE,
			custom_sub2api_enabled BOOLEAN NOT NULL DEFAULT FALSE,
			custom_sub2api_base_url TEXT NOT NULL DEFAULT '',
			custom_sub2api_api_key TEXT NOT NULL DEFAULT '',
			custom_sub2api_group_ids TEXT NOT NULL DEFAULT '[]',
			custom_sub2api_proxy_id TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
		`,
		`ALTER TABLE register_user_page_config ADD COLUMN IF NOT EXISTS herosms_api_key TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE register_user_page_config ADD COLUMN IF NOT EXISTS duck_authorization TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE register_user_page_config ADD COLUMN IF NOT EXISTS register_count INT NOT NULL DEFAULT 1`,
		`ALTER TABLE register_user_page_config ADD COLUMN IF NOT EXISTS email_count INT NOT NULL DEFAULT 1`,
		`ALTER TABLE register_user_page_config ADD COLUMN IF NOT EXISTS global_proxy TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE register_user_page_config ADD COLUMN IF NOT EXISTS proxy_sms_enabled BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE register_user_page_config ADD COLUMN IF NOT EXISTS proxy_openai_enabled BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE register_user_page_config ADD COLUMN IF NOT EXISTS proxy_email_enabled BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE register_user_page_config ADD COLUMN IF NOT EXISTS proxy_sub2api_enabled BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE register_user_page_config ADD COLUMN IF NOT EXISTS custom_sub2api_enabled BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE register_user_page_config ADD COLUMN IF NOT EXISTS custom_sub2api_base_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE register_user_page_config ADD COLUMN IF NOT EXISTS custom_sub2api_api_key TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE register_user_page_config ADD COLUMN IF NOT EXISTS custom_sub2api_group_ids TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE register_user_page_config ADD COLUMN IF NOT EXISTS custom_sub2api_proxy_id TEXT NOT NULL DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_user_email_user_unused ON user_email(user_id, used_at, id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_phone_accounts_user_status ON user_phone_accounts(user_id, status, id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_phone_accounts_phone ON user_phone_accounts(phone)`,
	}
	for _, statement := range statements {
		if _, err := r.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("ensure register schema: %w", err)
		}
	}
	return nil
}

func (r *Repository) GetRegisterUserByUsername(ctx context.Context, username string) (*RegisterUser, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("username 不能为空")
	}
	const q = `
		SELECT id, username, group_id, is_duck, COALESCE(otp_email, ''), password, created_at, updated_at
		FROM register_users
		WHERE LOWER(username) = LOWER($1)
		LIMIT 1
	`
	var user RegisterUser
	if err := r.db.QueryRowContext(ctx, q, username).Scan(&user.ID, &user.Username, &user.GroupID, &user.IsDuck, &user.OTPEmail, &user.Password, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("用户不存在: %s", username)
		}
		return nil, fmt.Errorf("query register_users: %w", err)
	}
	return &user, nil
}

func (r *Repository) GetRegisterUserByID(ctx context.Context, userID int64) (*RegisterUser, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("user_id 无效")
	}
	const q = `
		SELECT id, username, group_id, is_duck, COALESCE(otp_email, ''), password, created_at, updated_at
		FROM register_users
		WHERE id = $1
		LIMIT 1
	`
	var user RegisterUser
	if err := r.db.QueryRowContext(ctx, q, userID).Scan(&user.ID, &user.Username, &user.GroupID, &user.IsDuck, &user.OTPEmail, &user.Password, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("用户不存在: %d", userID)
		}
		return nil, fmt.Errorf("query register_users by id: %w", err)
	}
	return &user, nil
}

func (r *Repository) GetUserPageConfig(ctx context.Context, userID int64) (UserPageConfig, error) {
	if userID <= 0 {
		return UserPageConfig{}, fmt.Errorf("user_id 无效")
	}
	config := defaultUserPageConfig()
	const q = `
		SELECT
			COALESCE(herosms_api_key, ''),
			COALESCE(duck_authorization, ''),
			register_count,
			email_count,
			COALESCE(global_proxy, ''),
			proxy_sms_enabled,
			proxy_openai_enabled,
			proxy_email_enabled,
			proxy_sub2api_enabled,
			custom_sub2api_enabled,
			COALESCE(custom_sub2api_base_url, ''),
			COALESCE(custom_sub2api_api_key, ''),
			COALESCE(custom_sub2api_group_ids, '[]'),
			COALESCE(custom_sub2api_proxy_id, ''),
			updated_at
		FROM register_user_page_config
		WHERE user_id = $1
		LIMIT 1
	`
	groupIDsRaw := "[]"
	if err := r.db.QueryRowContext(ctx, q, userID).Scan(
		&config.HeroSMSAPIKey,
		&config.DuckAuthorization,
		&config.RegisterCount,
		&config.EmailCount,
		&config.GlobalProxy,
		&config.ProxySMSEnabled,
		&config.ProxyOpenAIEnabled,
		&config.ProxyEmailEnabled,
		&config.ProxySub2APIEnabled,
		&config.CustomSub2API.Enabled,
		&config.CustomSub2API.BaseURL,
		&config.CustomSub2API.APIKey,
		&groupIDsRaw,
		&config.CustomSub2API.ProxyID,
		&config.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return config, nil
		}
		return config, fmt.Errorf("query user page config: %w", err)
	}
	config.CustomSub2API.GroupIDs = parseConfigGroupIDs(groupIDsRaw)
	return normalizeUserPageConfig(config), nil
}

func (r *Repository) SaveUserPageConfig(ctx context.Context, userID int64, config UserPageConfig) (UserPageConfig, error) {
	if userID <= 0 {
		return UserPageConfig{}, fmt.Errorf("user_id 无效")
	}
	config = normalizeUserPageConfig(config)
	groupIDsRaw := "[]"
	if b, err := json.Marshal(positiveGroupIDs(config.CustomSub2API.GroupIDs)); err == nil {
		groupIDsRaw = string(b)
	}
	const q = `
		INSERT INTO register_user_page_config (
			user_id,
			herosms_api_key,
			duck_authorization,
			register_count,
			email_count,
			global_proxy,
			proxy_sms_enabled,
			proxy_openai_enabled,
			proxy_email_enabled,
			proxy_sub2api_enabled,
			custom_sub2api_enabled,
			custom_sub2api_base_url,
			custom_sub2api_api_key,
			custom_sub2api_group_ids,
			custom_sub2api_proxy_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (user_id) DO UPDATE
		SET herosms_api_key = EXCLUDED.herosms_api_key,
		    duck_authorization = EXCLUDED.duck_authorization,
		    register_count = EXCLUDED.register_count,
		    email_count = EXCLUDED.email_count,
		    global_proxy = EXCLUDED.global_proxy,
		    proxy_sms_enabled = EXCLUDED.proxy_sms_enabled,
		    proxy_openai_enabled = EXCLUDED.proxy_openai_enabled,
		    proxy_email_enabled = EXCLUDED.proxy_email_enabled,
		    proxy_sub2api_enabled = EXCLUDED.proxy_sub2api_enabled,
		    custom_sub2api_enabled = EXCLUDED.custom_sub2api_enabled,
		    custom_sub2api_base_url = EXCLUDED.custom_sub2api_base_url,
		    custom_sub2api_api_key = EXCLUDED.custom_sub2api_api_key,
		    custom_sub2api_group_ids = EXCLUDED.custom_sub2api_group_ids,
		    custom_sub2api_proxy_id = EXCLUDED.custom_sub2api_proxy_id,
		    updated_at = NOW()
		RETURNING updated_at
	`
	if err := r.db.QueryRowContext(
		ctx,
		q,
		userID,
		config.HeroSMSAPIKey,
		config.DuckAuthorization,
		config.RegisterCount,
		config.EmailCount,
		config.GlobalProxy,
		config.ProxySMSEnabled,
		config.ProxyOpenAIEnabled,
		config.ProxyEmailEnabled,
		config.ProxySub2APIEnabled,
		config.CustomSub2API.Enabled,
		config.CustomSub2API.BaseURL,
		config.CustomSub2API.APIKey,
		groupIDsRaw,
		config.CustomSub2API.ProxyID,
	).Scan(&config.UpdatedAt); err != nil {
		return UserPageConfig{}, fmt.Errorf("save user page config: %w", err)
	}
	return config, nil
}

func (r *Repository) UpdateRegisterUser(ctx context.Context, userID int64, otpEmail, password string) (*RegisterUser, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("user_id 无效")
	}
	otpEmail = strings.TrimSpace(otpEmail)
	password = strings.TrimSpace(password)
	const q = `
		UPDATE register_users
		SET otp_email = $2,
		    password = COALESCE(NULLIF($3, ''), password),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, username, group_id, is_duck, COALESCE(otp_email, ''), password, created_at, updated_at
	`
	var user RegisterUser
	if err := r.db.QueryRowContext(ctx, q, userID, otpEmail, password).Scan(&user.ID, &user.Username, &user.GroupID, &user.IsDuck, &user.OTPEmail, &user.Password, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("用户不存在: %d", userID)
		}
		return nil, fmt.Errorf("update register_users: %w", err)
	}
	return &user, nil
}

func (r *Repository) InsertUserEmail(ctx context.Context, userID int64, email, provider string, raw map[string]any) (bool, error) {
	email = strings.TrimSpace(email)
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = "manual"
	}
	if userID <= 0 || email == "" {
		return false, fmt.Errorf("user_id 和 email 不能为空")
	}
	rawText := "{}"
	if raw != nil {
		if b, err := json.Marshal(raw); err == nil {
			rawText = string(b)
		}
	}
	const q = `
		INSERT INTO user_email (user_id, email, provider, raw_response)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, email) DO NOTHING
	`
	result, err := r.db.ExecContext(ctx, q, userID, email, provider, rawText)
	if err != nil {
		return false, fmt.Errorf("insert user_email: %w", err)
	}
	affected, _ := result.RowsAffected()
	return affected > 0, nil
}

func (r *Repository) CountUserEmailsCreatedSince(ctx context.Context, userID int64, since time.Time) (int, error) {
	if userID <= 0 {
		return 0, fmt.Errorf("user_id 不能为空")
	}
	const q = `
		SELECT COUNT(*)
		FROM user_email
		WHERE user_id = $1
		  AND created_at >= $2
	`
	count := 0
	if err := r.db.QueryRowContext(ctx, q, userID, since).Scan(&count); err != nil {
		return 0, fmt.Errorf("count user_email created today: %w", err)
	}
	return count, nil
}

func (r *Repository) ClaimUnusedUserEmail(ctx context.Context, userID int64, exclude map[string]struct{}) (*UserEmail, error) {
	const q = `
		SELECT id, user_id, email, provider, used_at, COALESCE(account_id, 0), created_at, updated_at
		FROM user_email
		WHERE user_id = $1
		  AND used_at IS NULL
		ORDER BY id ASC
		LIMIT 100
	`
	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("query user_email: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		email := UserEmail{}
		var usedAt sql.NullTime
		if err := rows.Scan(&email.ID, &email.UserID, &email.Email, &email.Provider, &usedAt, &email.AccountID, &email.CreatedAt, &email.UpdatedAt); err != nil {
			return nil, err
		}
		if usedAt.Valid {
			email.UsedAt = &usedAt.Time
		}
		if _, ok := exclude[strings.ToLower(strings.TrimSpace(email.Email))]; ok {
			continue
		}
		return &email, nil
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (r *Repository) InsertUserPhoneAccount(ctx context.Context, session Session) (int64, error) {
	if session.UserID <= 0 || strings.TrimSpace(session.Phone) == "" || strings.TrimSpace(session.Password) == "" {
		return 0, fmt.Errorf("用户账号缺少 user_id、phone 或 password")
	}
	const q = `
		INSERT INTO user_phone_accounts (
			user_id, phone, password, name, birthdate, status, phone_session_id
		)
		VALUES ($1, $2, $3, $4, $5, 'registered', $6)
		RETURNING id
	`
	var id int64
	if err := r.db.QueryRowContext(ctx, q, session.UserID, session.Phone, session.Password, session.Name, session.Birthdate, session.ID).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert user_phone_accounts: %w", err)
	}
	return id, nil
}

func (r *Repository) UpdateUserAccountStatus(ctx context.Context, accountID int64, status, errText string) error {
	if accountID <= 0 {
		return fmt.Errorf("account_id 无效")
	}
	const q = `
		UPDATE user_phone_accounts
		SET status = $2,
		    error = $3,
		    updated_at = NOW()
		WHERE id = $1
	`
	if _, err := r.db.ExecContext(ctx, q, accountID, strings.TrimSpace(status), strings.TrimSpace(errText)); err != nil {
		return fmt.Errorf("update user_phone_accounts status: %w", err)
	}
	return nil
}

func (r *Repository) AttachUserEmailToAccount(ctx context.Context, accountID int64, email UserEmail) error {
	if accountID <= 0 || email.ID <= 0 || strings.TrimSpace(email.Email) == "" {
		return fmt.Errorf("account_id 或 user_email 无效")
	}
	const q = `
		UPDATE user_phone_accounts
		SET user_email_id = $2,
		    email = $3,
		    updated_at = NOW()
		WHERE id = $1
	`
	if _, err := r.db.ExecContext(ctx, q, accountID, email.ID, email.Email); err != nil {
		return fmt.Errorf("attach user_email to account: %w", err)
	}
	return nil
}

func (r *Repository) SaveUserPhoneAccountToken(
	ctx context.Context,
	accountID int64,
	email UserEmail,
	tokenData map[string]any,
	sub2apiJSON map[string]any,
) map[string]any {
	if accountID <= 0 || email.ID <= 0 || strings.TrimSpace(email.Email) == "" {
		return map[string]any{"ok": false, "error": "account_id 或 user_email 无效"}
	}
	tokenRaw, err := json.Marshal(tokenData)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error(), "stage": "marshal_token"}
	}
	sub2apiRaw, err := json.Marshal(sub2apiJSON)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error(), "stage": "marshal_sub2api"}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error(), "stage": "begin"}
	}
	defer tx.Rollback()
	const accountQ = `
		UPDATE user_phone_accounts
		SET user_email_id = $2,
		    email = $3,
		    status = 'uploading',
		    error = '',
		    token = $4,
		    sub2api_json = $5,
		    updated_at = NOW()
		WHERE id = $1
	`
	if _, err := tx.ExecContext(ctx, accountQ, accountID, email.ID, email.Email, string(tokenRaw), string(sub2apiRaw)); err != nil {
		return map[string]any{"ok": false, "error": err.Error(), "stage": "account"}
	}
	const emailQ = `
		UPDATE user_email
		SET used_at = COALESCE(used_at, NOW()),
		    account_id = $2,
		    updated_at = NOW()
		WHERE id = $1
	`
	if _, err := tx.ExecContext(ctx, emailQ, email.ID, accountID); err != nil {
		return map[string]any{"ok": false, "error": err.Error(), "stage": "email"}
	}
	if err := tx.Commit(); err != nil {
		return map[string]any{"ok": false, "error": err.Error(), "stage": "commit"}
	}
	return map[string]any{"ok": true, "stage": "user_account_token_saved", "account_id": accountID, "user_email_id": email.ID}
}

func (r *Repository) FinalizeUserPhoneAccount(
	ctx context.Context,
	accountID int64,
	email UserEmail,
	tokenData map[string]any,
	sub2apiJSON map[string]any,
	uploadResult map[string]any,
) map[string]any {
	if accountID <= 0 || email.ID <= 0 || strings.TrimSpace(email.Email) == "" {
		return map[string]any{"ok": false, "error": "account_id 或 user_email 无效"}
	}
	tokenRaw, err := json.Marshal(tokenData)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error(), "stage": "marshal_token"}
	}
	sub2apiRaw, err := json.Marshal(sub2apiJSON)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error(), "stage": "marshal_sub2api"}
	}
	uploadRaw, err := json.Marshal(uploadResult)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error(), "stage": "marshal_upload"}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error(), "stage": "begin"}
	}
	defer tx.Rollback()
	const accountQ = `
		UPDATE user_phone_accounts
		SET user_email_id = $2,
		    email = $3,
		    status = 'success',
		    error = '',
		    token = $4,
		    sub2api_json = $5,
		    sub2api_upload_result = $6,
		    updated_at = NOW()
		WHERE id = $1
	`
	if _, err := tx.ExecContext(ctx, accountQ, accountID, email.ID, email.Email, string(tokenRaw), string(sub2apiRaw), string(uploadRaw)); err != nil {
		return map[string]any{"ok": false, "error": err.Error(), "stage": "account"}
	}
	const emailQ = `
		UPDATE user_email
		SET used_at = COALESCE(used_at, NOW()),
		    account_id = $2,
		    updated_at = NOW()
		WHERE id = $1
	`
	if _, err := tx.ExecContext(ctx, emailQ, email.ID, accountID); err != nil {
		return map[string]any{"ok": false, "error": err.Error(), "stage": "email"}
	}
	if err := tx.Commit(); err != nil {
		return map[string]any{"ok": false, "error": err.Error(), "stage": "commit"}
	}
	return map[string]any{"ok": true, "stage": "user_account_finalized", "account_id": accountID, "user_email_id": email.ID}
}

func (r *Repository) GetUserAccountSub2APIUploadTarget(ctx context.Context, userID, accountID int64) (*UserAccountSub2APIUploadTarget, error) {
	if userID <= 0 || accountID <= 0 {
		return nil, fmt.Errorf("user_id 或 account_id 无效")
	}
	const q = `
		SELECT
			a.id,
			a.user_id,
			u.group_id,
			COALESCE(a.email, ''),
			COALESCE(a.sub2api_json, '{}'),
			COALESCE(a.sub2api_upload_result, '{}')
		FROM user_phone_accounts a
		JOIN register_users u ON u.id = a.user_id
		WHERE a.user_id = $1
		  AND a.id = $2
		LIMIT 1
	`
	var target UserAccountSub2APIUploadTarget
	var sub2apiRaw, uploadRaw string
	if err := r.db.QueryRowContext(ctx, q, userID, accountID).Scan(
		&target.ID,
		&target.UserID,
		&target.UserGroupID,
		&target.Email,
		&sub2apiRaw,
		&uploadRaw,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("账号不存在或不属于当前用户")
		}
		return nil, fmt.Errorf("query user account sub2api: %w", err)
	}
	if err := json.Unmarshal([]byte(firstString(sub2apiRaw, "{}")), &target.Sub2APIJSON); err != nil {
		return nil, fmt.Errorf("解析账号 Sub2API JSON 失败: %w", err)
	}
	if err := json.Unmarshal([]byte(firstString(uploadRaw, "{}")), &target.UploadResult); err != nil {
		target.UploadResult = map[string]any{}
	}
	return &target, nil
}

func (r *Repository) SaveUserAccountSub2APIUploadResult(ctx context.Context, userID, accountID int64, sub2apiJSON, uploadResult map[string]any) error {
	if userID <= 0 || accountID <= 0 {
		return fmt.Errorf("user_id 或 account_id 无效")
	}
	sub2apiRaw, err := json.Marshal(sub2apiJSON)
	if err != nil {
		return fmt.Errorf("marshal sub2api json: %w", err)
	}
	uploadRaw, err := json.Marshal(uploadResult)
	if err != nil {
		return fmt.Errorf("marshal upload result: %w", err)
	}
	const q = `
		UPDATE user_phone_accounts
		SET sub2api_json = $3,
		    sub2api_upload_result = $4,
		    status = 'success',
		    error = '',
		    updated_at = NOW()
		WHERE user_id = $1
		  AND id = $2
	`
	result, err := r.db.ExecContext(ctx, q, userID, accountID, string(sub2apiRaw), string(uploadRaw))
	if err != nil {
		return fmt.Errorf("save user account sub2api upload result: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("账号不存在或不属于当前用户")
	}
	return nil
}

// MarkUserEmailUsed 把 user_email 标记为已使用（used_at），并在尚未关联账号时补上
// account_id。用于「邮箱已绑定账号、但后续 token 步骤失败」的场景：即便整体失败，也要
// 占用该邮箱，避免下次注册重复选到这个已绑定到失败账号的邮箱。used_at 用 COALESCE 保留
// 首次时间，重复调用安全幂等。
func (r *Repository) MarkUserEmailUsed(ctx context.Context, emailID, accountID int64) error {
	if emailID <= 0 {
		return fmt.Errorf("user_email id 无效")
	}
	const q = `
		UPDATE user_email
		SET used_at = COALESCE(used_at, NOW()),
		    account_id = COALESCE(account_id, NULLIF($2, 0)),
		    updated_at = NOW()
		WHERE id = $1
	`
	if _, err := r.db.ExecContext(ctx, q, emailID, accountID); err != nil {
		return fmt.Errorf("mark user_email used: %w", err)
	}
	return nil
}

func (r *Repository) UserSummary(ctx context.Context, userID int64) (UserSummary, error) {
	summary := UserSummary{}
	const emailQ = `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE used_at IS NULL),
			COUNT(*) FILTER (WHERE used_at IS NOT NULL)
		FROM user_email
		WHERE user_id = $1
	`
	if err := r.db.QueryRowContext(ctx, emailQ, userID).Scan(&summary.EmailTotal, &summary.EmailUnused, &summary.EmailUsed); err != nil {
		return summary, fmt.Errorf("query user email summary: %w", err)
	}
	const accountQ = `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'queued_login'),
			COUNT(*) FILTER (WHERE status IN ('login_running', 'uploading')),
			COUNT(*) FILTER (WHERE status = 'success'),
			COUNT(*) FILTER (WHERE status = 'failed'),
			COUNT(*) FILTER (WHERE status = 'registered')
		FROM user_phone_accounts
		WHERE user_id = $1
	`
	if err := r.db.QueryRowContext(ctx, accountQ, userID).Scan(
		&summary.AccountTotal,
		&summary.AccountQueued,
		&summary.AccountRunning,
		&summary.AccountSuccess,
		&summary.AccountFailed,
		&summary.AccountRegistered,
	); err != nil {
		return summary, fmt.Errorf("query user account summary: %w", err)
	}
	return summary, nil
}

func (r *Repository) UserLoginSummary(ctx context.Context, userID int64) (LoginSummary, error) {
	summary := LoginSummary{}
	if userID <= 0 {
		return summary, fmt.Errorf("user_id 无效")
	}
	const q = `
		SELECT
			COUNT(*) FILTER (WHERE status = 'queued_login'),
			COUNT(*) FILTER (WHERE status IN ('login_running', 'uploading')),
			COUNT(*) FILTER (WHERE status = 'success'),
			COUNT(*) FILTER (WHERE status = 'failed'),
			COUNT(*) FILTER (WHERE status IN ('queued_login', 'login_running', 'uploading', 'success', 'failed'))
		FROM user_phone_accounts
		WHERE user_id = $1
		  AND status IN ('queued_login', 'login_running', 'uploading', 'success', 'failed')
	`
	if err := r.db.QueryRowContext(ctx, q, userID).Scan(
		&summary.Queued,
		&summary.Running,
		&summary.Success,
		&summary.Failed,
		&summary.Total,
	); err != nil {
		return summary, fmt.Errorf("query user login summary: %w", err)
	}
	return summary, nil
}

func (r *Repository) LatestUserAccounts(ctx context.Context, userID int64, limit int) ([]UserAccount, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	const q = `
		SELECT id, user_id, phone, COALESCE(email, ''), password, status, COALESCE(error, ''), created_at, updated_at
		FROM user_phone_accounts
		WHERE user_id = $1
		ORDER BY id DESC
		LIMIT $2
	`
	rows, err := r.db.QueryContext(ctx, q, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query latest user accounts: %w", err)
	}
	defer rows.Close()
	out := []UserAccount{}
	for rows.Next() {
		var item UserAccount
		if err := rows.Scan(&item.ID, &item.UserID, &item.Phone, &item.Email, &item.Password, &item.Status, &item.Error, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) ListUserEmails(ctx context.Context, userID int64, page, pageSize int, search string) (UserEmailListResponse, error) {
	if userID <= 0 {
		return UserEmailListResponse{}, fmt.Errorf("user_id 无效")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize
	resp := UserEmailListResponse{
		Items:    []UserEmailListItem{},
		Page:     page,
		PageSize: pageSize,
	}

	search = strings.TrimSpace(search)
	phoneSearch := normalizePhoneSearch(search)
	where := `WHERE e.user_id = $1`
	args := []any{userID}
	if search != "" {
		where += `
			AND (
				e.email ILIKE $2
				OR ($3 <> '' AND regexp_replace(COALESCE(a.phone, ''), '[+()[:space:]-]', '', 'g') ILIKE $4)
			)
		`
		args = append(args, "%"+search+"%", phoneSearch, "%"+phoneSearch+"%")
	}

	countQ := `
		SELECT COUNT(*)
		FROM user_email e
		LEFT JOIN LATERAL (
			SELECT phone
			FROM user_phone_accounts a
			WHERE a.user_id = e.user_id
			  AND (a.id = e.account_id OR a.user_email_id = e.id)
			ORDER BY
				CASE WHEN a.id = e.account_id THEN 0 ELSE 1 END,
				a.id DESC
			LIMIT 1
		) a ON TRUE
		` + where
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&resp.Total); err != nil {
		return resp, fmt.Errorf("count user emails: %w", err)
	}
	if resp.Total > 0 {
		resp.TotalPages = (resp.Total + pageSize - 1) / pageSize
	}
	q := `
		SELECT
			e.id,
			e.user_id,
			e.email,
			e.provider,
			e.used_at,
			COALESCE(NULLIF(e.account_id, 0), a.id, 0),
			COALESCE(a.phone, ''),
			COALESCE(a.status, ''),
			COALESCE(a.error, ''),
			CASE
				WHEN a.id IS NULL THEN FALSE
				WHEN COALESCE(a.sub2api_json, '{}') <> '{}' THEN TRUE
				ELSE FALSE
			END,
			CASE
				WHEN a.id IS NULL THEN FALSE
				WHEN COALESCE(a.sub2api_upload_result, '{}') <> '{}' THEN TRUE
				ELSE FALSE
			END,
			e.created_at,
			e.updated_at
		FROM user_email e
		LEFT JOIN LATERAL (
			SELECT id, phone, status, error, sub2api_json, sub2api_upload_result
			FROM user_phone_accounts a
			WHERE a.user_id = e.user_id
			  AND (a.id = e.account_id OR a.user_email_id = e.id)
			ORDER BY
				CASE WHEN a.id = e.account_id THEN 0 ELSE 1 END,
				a.id DESC
			LIMIT 1
		) a ON TRUE
		` + where + `
		ORDER BY e.id DESC
		LIMIT $` + fmt.Sprint(len(args)+1) + ` OFFSET $` + fmt.Sprint(len(args)+2) + `
	`
	args = append(args, pageSize, offset)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return resp, fmt.Errorf("query user emails: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		item := UserEmailListItem{}
		var usedAt sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Email,
			&item.Provider,
			&usedAt,
			&item.AccountID,
			&item.Phone,
			&item.AccountStatus,
			&item.AccountError,
			&item.Sub2APIReady,
			&item.Sub2APIUploaded,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return resp, err
		}
		if usedAt.Valid {
			item.UsedAt = &usedAt.Time
		}
		resp.Items = append(resp.Items, item)
	}
	if err := rows.Err(); err != nil {
		return resp, err
	}
	return resp, nil
}

func normalizePhoneSearch(value string) string {
	replacer := strings.NewReplacer("+", "", "(", "", ")", "", " ", "", "-", "")
	return replacer.Replace(strings.TrimSpace(value))
}

func defaultUserPageConfig() UserPageConfig {
	return UserPageConfig{
		RegisterCount: 1,
		EmailCount:    1,
		CustomSub2API: CustomSub2APIConfig{GroupIDs: []int64{}},
	}
}

func normalizeUserPageConfig(config UserPageConfig) UserPageConfig {
	config.HeroSMSAPIKey = strings.TrimSpace(config.HeroSMSAPIKey)
	config.DuckAuthorization = strings.TrimSpace(config.DuckAuthorization)
	config.GlobalProxy = strings.TrimSpace(config.GlobalProxy)
	if config.RegisterCount <= 0 {
		config.RegisterCount = 1
	}
	if config.RegisterCount > maxHeroSMSBatchCount {
		config.RegisterCount = maxHeroSMSBatchCount
	}
	if config.EmailCount <= 0 {
		config.EmailCount = 1
	}
	if config.EmailCount > userEmailDailyCreateLimit {
		config.EmailCount = userEmailDailyCreateLimit
	}
	config.CustomSub2API.BaseURL = strings.TrimSpace(config.CustomSub2API.BaseURL)
	config.CustomSub2API.APIKey = strings.TrimSpace(config.CustomSub2API.APIKey)
	config.CustomSub2API.GroupIDs = positiveGroupIDs(config.CustomSub2API.GroupIDs)
	config.CustomSub2API.ProxyID = strings.TrimSpace(config.CustomSub2API.ProxyID)
	return config
}

func parseConfigGroupIDs(raw string) []int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var ids []int64
	if err := json.Unmarshal([]byte(raw), &ids); err == nil {
		return positiveGroupIDs(ids)
	}
	var mixed []any
	if err := json.Unmarshal([]byte(raw), &mixed); err == nil {
		out := make([]int64, 0, len(mixed))
		for _, item := range mixed {
			switch value := item.(type) {
			case float64:
				out = append(out, int64(value))
			case string:
				if id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
					out = append(out, id)
				}
			}
		}
		return positiveGroupIDs(out)
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；' || r == ' ' || r == '\n' || r == '\t'
	})
	out := make([]int64, 0, len(parts))
	for _, part := range parts {
		if id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil {
			out = append(out, id)
		}
	}
	return positiveGroupIDs(out)
}

func (r *Repository) PhoneExists(ctx context.Context, phone string) (bool, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return false, nil
	}
	const q = `
		SELECT 1
		FROM user_phone_accounts
		WHERE phone = $1
		  AND status <> 'failed'
		LIMIT 1
	`
	var marker int
	err := r.db.QueryRowContext(ctx, q, phone).Scan(&marker)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query user_phone_accounts by phone: %w", err)
	}
	return true, nil
}
