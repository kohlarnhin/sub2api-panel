package register

import "time"

const (
	DefaultHeroSMSService       = "dr"
	DefaultHeroSMSCountry       = 151
	DefaultHeroSMSOperator      = "any"
	DefaultHeroSMSMaxPrice      = 0.03
	DefaultHeroSMSOwner         = 6
	DefaultHeroSMSActivation    = 0
	DefaultHeroSMSAmount        = 1
	DefaultHeroSMSTemplateName  = "智利"
	DefaultRegisterUserPassword = "5nuEGNrh7h4km5aTAy81"
)

type HeroSMSTemplate struct {
	Name           string  `json:"name"`
	Service        string  `json:"service"`
	Country        int     `json:"country"`
	Operator       string  `json:"operator"`
	MaxPrice       float64 `json:"max_price"`
	FixedPrice     bool    `json:"fixed_price"`
	Owner          int     `json:"owner"`
	ActivationType int     `json:"activation_type"`
	Amount         int     `json:"amount"`
	Enabled        bool    `json:"enabled"`
	SortOrder      int     `json:"sort_order"`
}

type StartRequest struct {
	APIKey   string  `json:"api_key" binding:"required"`
	Count    int     `json:"count"`
	GroupIDs []int64 `json:"group_ids"`
}

type UserLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UserUpdateRequest struct {
	UserID          int64  `json:"user_id" binding:"required"`
	OTPEmail        string `json:"otp_email"`
	Password        string `json:"password"`
	CurrentPassword string `json:"current_password"`
}

type UserEmailGenerateRequest struct {
	UserID            int64  `json:"user_id" binding:"required"`
	Count             int    `json:"count"`
	DuckAuthorization string `json:"duck_authorization"`
}

type UserSub2APIUploadRequest struct {
	UserID        int64                `json:"user_id" binding:"required"`
	AccountID     int64                `json:"account_id" binding:"required"`
	CustomSub2API *CustomSub2APIConfig `json:"custom_sub2api,omitempty"`
}

type UserRegisterStartRequest struct {
	UserID        int64                `json:"user_id" binding:"required"`
	APIKey        string               `json:"api_key" binding:"required"`
	Count         int                  `json:"count"`
	CustomSub2API *CustomSub2APIConfig `json:"custom_sub2api,omitempty"`
}

type CustomSub2APIConfig struct {
	Enabled  bool    `json:"enabled"`
	BaseURL  string  `json:"base_url"`
	APIKey   string  `json:"api_key"`
	GroupIDs []int64 `json:"group_ids"`
	ProxyID  string  `json:"proxy_id"`
}

type StopRequest struct {
	SessionID string `json:"session_id" binding:"required"`
}

type UserStopRequest struct {
	UserID int64 `json:"user_id" binding:"required"`
}

type Session struct {
	ID                   string                 `json:"session_id"`
	UserID               int64                  `json:"user_id,omitempty"`
	UserName             string                 `json:"username,omitempty"`
	OTPMailbox           string                 `json:"otp_email,omitempty"`
	UserEmailID          int64                  `json:"user_email_id,omitempty"`
	AccountID            int64                  `json:"account_id,omitempty"`
	RunID                string                 `json:"run_id,omitempty"`
	Phone                string                 `json:"phone"`
	Email                string                 `json:"email"`
	Password             string                 `json:"password"`
	Name                 string                 `json:"name"`
	Birthdate            string                 `json:"birthdate"`
	Status               string                 `json:"status"`
	Step                 string                 `json:"step"`
	Error                string                 `json:"error"`
	RawResponse          map[string]any         `json:"raw_response,omitempty"`
	Sub2APIJSON          map[string]any         `json:"sub2api_json,omitempty"`
	Sub2APIUploadResult  map[string]any         `json:"sub2api_upload_result,omitempty"`
	DatabaseSaveResult   map[string]any         `json:"database_save_result,omitempty"`
	HeroSMSActivationID  string                 `json:"hero_sms_activation_id"`
	HeroSMSAttempt       int                    `json:"hero_sms_attempt"`
	HeroSMSAttempts      []HeroSMSAttempt       `json:"hero_sms_attempts"`
	Template             HeroSMSTemplate        `json:"template"`
	GroupIDs             []int64                `json:"group_ids"`
	StopRequested        bool                   `json:"stop_requested"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
	AdditionalAttributes map[string]interface{} `json:"additional_attributes,omitempty"`
}

type HeroSMSAttempt struct {
	Attempt      int            `json:"attempt"`
	ActivationID string         `json:"activation_id"`
	Phone        string         `json:"phone"`
	Number       map[string]any `json:"number,omitempty"`
	LastStatus   map[string]any `json:"last_status,omitempty"`
}

type Batch struct {
	ID               string         `json:"batch_id"`
	TargetCount      int            `json:"target_count"`
	Status           string         `json:"status"`
	Step             string         `json:"step"`
	Error            string         `json:"error"`
	SuccessCount     int            `json:"success_count"`
	FailedCount      int            `json:"failed_count"`
	CurrentSessionID string         `json:"current_session_id"`
	SessionIDs       []string       `json:"session_ids"`
	Results          []BatchResult  `json:"results"`
	Failures         []BatchFailure `json:"failures"`
	CurrentSession   *Session       `json:"current_session,omitempty"`
	StopRequested    bool           `json:"stop_requested"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type BatchResult struct {
	SessionID string `json:"session_id"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Step      string `json:"step"`
}

type BatchFailure struct {
	SessionID string `json:"session_id"`
	Phone     string `json:"phone,omitempty"`
	Email     string `json:"email,omitempty"`
	Step      string `json:"step"`
	Error     string `json:"error"`
}

type RegisterUser struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	GroupID   int64     `json:"group_id"`
	IsDuck    bool      `json:"is_duck"`
	OTPEmail  string    `json:"otp_email"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserEmail struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"user_id"`
	Email     string     `json:"email"`
	Provider  string     `json:"provider"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	AccountID int64      `json:"account_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type UserEmailListItem struct {
	ID              int64      `json:"id"`
	UserID          int64      `json:"user_id"`
	Email           string     `json:"email"`
	Provider        string     `json:"provider"`
	UsedAt          *time.Time `json:"used_at,omitempty"`
	AccountID       int64      `json:"account_id,omitempty"`
	Phone           string     `json:"phone"`
	AccountStatus   string     `json:"account_status"`
	AccountError    string     `json:"account_error,omitempty"`
	Sub2APIReady    bool       `json:"sub2api_ready"`
	Sub2APIUploaded bool       `json:"sub2api_uploaded"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type UserEmailListResponse struct {
	Items      []UserEmailListItem `json:"items"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
	Total      int                 `json:"total"`
	TotalPages int                 `json:"total_pages"`
}

type UserAccount struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email"`
	Password  string    `json:"password"`
	Status    string    `json:"status"`
	Error     string    `json:"error"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserAccountSub2APIUploadTarget struct {
	ID           int64          `json:"id"`
	UserID       int64          `json:"user_id"`
	UserGroupID  int64          `json:"user_group_id"`
	Email        string         `json:"email"`
	Sub2APIJSON  map[string]any `json:"sub2api_json"`
	UploadResult map[string]any `json:"sub2api_upload_result,omitempty"`
}

type UserSummary struct {
	EmailTotal        int `json:"email_total"`
	EmailUnused       int `json:"email_unused"`
	EmailUsed         int `json:"email_used"`
	AccountTotal      int `json:"account_total"`
	AccountQueued     int `json:"account_queued"`
	AccountRunning    int `json:"account_running"`
	AccountSuccess    int `json:"account_success"`
	AccountFailed     int `json:"account_failed"`
	AccountRegistered int `json:"account_registered"`
}

type LoginSummary struct {
	Queued  int `json:"queued"`
	Running int `json:"running"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
	Total   int `json:"total"`
}

type UserRunLog struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

type UserRun struct {
	ID                    string       `json:"run_id"`
	UserID                int64        `json:"user_id"`
	Username              string       `json:"username"`
	TargetCount           int          `json:"target_count"`
	Status                string       `json:"status"`
	Step                  string       `json:"step"`
	Error                 string       `json:"error"`
	PhoneSuccessCount     int          `json:"phone_success_count"`
	PhoneFailureCount     int          `json:"phone_failure_count"`
	LoginQueuedCount      int          `json:"login_queued_count"`
	LoginStartedCount     int          `json:"login_started_count"`
	LoginSuccessCount     int          `json:"login_success_count"`
	LoginFailedCount      int          `json:"login_failed_count"`
	PhoneCodeAttempt      int          `json:"phone_code_attempt"`
	PhoneCodeMaxAttempts  int          `json:"phone_code_max_attempts"`
	LoginEmailCodeAttempt int          `json:"login_email_code_attempt"`
	LoginEmailCodeMax     int          `json:"login_email_code_max"`
	CurrentSessionID      string       `json:"current_session_id"`
	CurrentPhone          string       `json:"current_phone"`
	CurrentAccountID      int64        `json:"current_account_id"`
	CurrentLoginAccountID int64        `json:"current_login_account_id"`
	PhoneDone             bool         `json:"phone_done"`
	StopRequested         bool         `json:"stop_requested"`
	Logs                  []UserRunLog `json:"logs"`
	CreatedAt             time.Time    `json:"created_at"`
	UpdatedAt             time.Time    `json:"updated_at"`
}

type UserDashboard struct {
	User           RegisterUser  `json:"user"`
	Summary        UserSummary   `json:"summary"`
	LoginSummary   LoginSummary  `json:"login_summary"`
	Run            *UserRun      `json:"run,omitempty"`
	EmailRun       *UserEmailRun `json:"email_run,omitempty"`
	LatestAccounts []UserAccount `json:"latest_accounts"`
}

type UserEmailGenerateResult struct {
	Target      int         `json:"target"`
	Created     int         `json:"created"`
	Attempts    int         `json:"attempts"`
	Skipped     int         `json:"skipped"`
	Failed      int         `json:"failed"`
	MaxAttempts int         `json:"max_attempts"`
	Emails      []string    `json:"emails"`
	Errors      []string    `json:"errors"`
	Summary     UserSummary `json:"summary"`
}

type UserEmailRun struct {
	UserID      int64        `json:"user_id"`
	Username    string       `json:"username"`
	Target      int          `json:"target"`
	Created     int          `json:"created"`
	Attempts    int          `json:"attempts"`
	Skipped     int          `json:"skipped"`
	Failed      int          `json:"failed"`
	MaxAttempts int          `json:"max_attempts"`
	Status      string       `json:"status"`
	Step        string       `json:"step"`
	Error       string       `json:"error"`
	LastEmail   string       `json:"last_email"`
	LastError   string       `json:"last_error"`
	Logs        []UserRunLog `json:"logs"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}
