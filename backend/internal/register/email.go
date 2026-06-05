package register

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultEmailPollAttempts = 60
	defaultEmailPollInterval = 2 * time.Second
	emailHTTPTimeout         = 15 * time.Second
	// emailAutoFetchBudget 为后台自动获取+确认邮箱验证码的总时长上限，
	// 需大于「轮询时长 + Codex 续接」所需时间，与 worker tryCtx 预算一致。
	emailAutoFetchBudget = 7 * time.Minute
)

// 验证码匹配：优先 XXX-XXX（去横杠后即 6 位），否则纯 6 位数字。与参考项目 EmailService 一致。
var (
	emailCodeWithDashRe = regexp.MustCompile(`(?i)\b([A-Z0-9]{3}-[A-Z0-9]{3})\b`)
	emailCodeDigitsRe   = regexp.MustCompile(`\b([0-9]{6})\b`)
	emailCodeContextRe  = regexp.MustCompile(`(?is)(?:临时验证码|验证码|verification code|temporary code|one[- ]time code|security code)[\s\S]{0,500}?([0-9]{6}|[A-Z0-9]{3}-[A-Z0-9]{3})`)
	htmlNoiseRe         = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>|<script[^>]*>.*?</script>`)
	htmlCommentRe       = regexp.MustCompile(`(?is)<!--.*?-->`)
	htmlTagRe           = regexp.MustCompile(`<[^>]+>`)
	htmlTextNodeCodeRe  = regexp.MustCompile(`(?is)>\s*([0-9]{6})\s*<`)
)

// EmailClient 通过 freemail (Cloudflare Email Worker) 的 HTTP API 读取 OTP 邮箱，
// 自动获取 OpenAI 绑定邮箱时下发的验证码。对应参考项目的 EmailService。
type EmailClient struct {
	baseURL    string // 不含协议前缀的 worker 域名
	token      string
	mailbox    string
	attempts   int
	interval   time.Duration
	httpClient *http.Client
}

// NewEmailClient 构造邮箱 OTP 客户端。workerDomain 可带或不带 https:// 前缀。
func NewEmailClient(workerDomain, token, mailbox string, attempts int, interval time.Duration) *EmailClient {
	domain := strings.TrimSpace(workerDomain)
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")
	if attempts <= 0 {
		attempts = defaultEmailPollAttempts
	}
	if interval <= 0 {
		interval = defaultEmailPollInterval
	}
	return &EmailClient{
		baseURL:    domain,
		token:      strings.TrimSpace(token),
		mailbox:    strings.TrimSpace(mailbox),
		attempts:   attempts,
		interval:   interval,
		httpClient: &http.Client{Timeout: emailHTTPTimeout},
	}
}

// Configured 仅当 worker 域名和 token 配置后才启用自动获取；收件箱按用户动态传入。
func (c *EmailClient) Configured() bool {
	return c != nil && c.baseURL != "" && c.token != ""
}

func (c *EmailClient) ConfiguredForMailbox(mailbox string) bool {
	return c.Configured() && strings.TrimSpace(mailbox) != ""
}

// LatestEmailID 返回 OTP 邮箱当前最新邮件 id，作为后续轮询的基线（只取 id 更大的新邮件）。
func (c *EmailClient) LatestEmailIDForMailbox(ctx context.Context, mailbox string) int {
	items, err := c.fetchMailItems(ctx, strings.TrimSpace(mailbox))
	if err != nil || len(items) == 0 {
		return 0
	}
	return mailItemID(items[0])
}

// FetchVerificationCode 轮询 OTP 邮箱，返回 id 大于 minID 的新邮件中的验证码。
// 当 ctx 有 deadline 时会一直轮询到 ctx 结束；没有 deadline 时才使用 attempts 作为兜底上限。
// stop 返回 true 时立即中止等待。
func (c *EmailClient) FetchVerificationCodeForMailbox(ctx context.Context, mailbox string, minID int, stop func() bool, progress func(int, error)) (string, error) {
	_, hasDeadline := ctx.Deadline()
	maxAttempts := c.attempts
	if hasDeadline {
		maxAttempts = 0
	}
	return c.fetchVerificationCode(ctx, strings.TrimSpace(mailbox), minID, maxAttempts, stop, progress)
}

// FetchVerificationCodeAttempts 轮询固定次数后返回空验证码，供上层触发重新发送 OTP。
func (c *EmailClient) FetchVerificationCodeAttemptsForMailbox(ctx context.Context, mailbox string, minID, attempts int, stop func() bool, progress func(int, error)) (string, error) {
	return c.fetchVerificationCode(ctx, strings.TrimSpace(mailbox), minID, attempts, stop, progress)
}

func (c *EmailClient) fetchVerificationCode(ctx context.Context, mailbox string, minID, attempts int, stop func() bool, progress func(int, error)) (string, error) {
	if strings.TrimSpace(mailbox) == "" {
		return "", fmt.Errorf("OTP 邮箱未配置")
	}
	for attempt := 1; ; attempt++ {
		if attempts > 0 && attempt > attempts {
			return "", nil
		}
		if stop != nil && stop() {
			return "", fmt.Errorf("已停止等待邮箱验证码")
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		items, err := c.fetchMailItems(ctx, mailbox)
		if err == nil {
			for _, item := range items {
				itemID := mailItemID(item)
				if itemID <= minID {
					continue
				}
				if code := extractEmailCode(item); code != "" {
					return code, nil
				}
				detail, detailErr := c.fetchMailDetail(ctx, mailbox, itemID)
				if detailErr == nil {
					if code := extractEmailCode(detail); code != "" {
						return code, nil
					}
				}
			}
		}
		if progress != nil {
			progress(attempt, err)
		}
		if stop != nil && stop() {
			return "", fmt.Errorf("已停止等待邮箱验证码")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(c.interval):
		}
	}
}

// fetchMailItems 拉取 OTP 邮箱邮件列表，按 id 倒序排序。
func (c *EmailClient) fetchMailItems(ctx context.Context, mailbox string) ([]map[string]any, error) {
	mailbox = strings.TrimSpace(mailbox)
	if mailbox == "" {
		return nil, fmt.Errorf("OTP 邮箱未配置")
	}
	endpoint := fmt.Sprintf("https://%s/api/emails?mailbox=%s", c.baseURL, url.QueryEscape(mailbox))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("freemail http=%d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	items, err := parseMailItems(body)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		return mailItemID(items[i]) > mailItemID(items[j])
	})
	return items, nil
}

func (c *EmailClient) fetchMailDetail(ctx context.Context, mailbox string, id int) (map[string]any, error) {
	if id <= 0 {
		return nil, fmt.Errorf("email id 无效")
	}
	mailbox = strings.TrimSpace(mailbox)
	if mailbox == "" {
		return nil, fmt.Errorf("OTP 邮箱未配置")
	}
	endpoints := []string{
		fmt.Sprintf("https://%s/api/email/%d", c.baseURL, id),
		fmt.Sprintf("https://%s/api/emails/%d?mailbox=%s", c.baseURL, id, url.QueryEscape(mailbox)),
		fmt.Sprintf("https://%s/api/emails/%d?address=%s", c.baseURL, id, url.QueryEscape(mailbox)),
		fmt.Sprintf("https://%s/api/emails/%d", c.baseURL, id),
		fmt.Sprintf("https://%s/api/emails?id=%d&mailbox=%s", c.baseURL, id, url.QueryEscape(mailbox)),
		fmt.Sprintf("https://%s/api/emails?id=%d&address=%s", c.baseURL, id, url.QueryEscape(mailbox)),
		fmt.Sprintf("https://%s/api/email?id=%d&mailbox=%s", c.baseURL, id, url.QueryEscape(mailbox)),
		fmt.Sprintf("https://%s/api/email?id=%d&address=%s", c.baseURL, id, url.QueryEscape(mailbox)),
		fmt.Sprintf("https://%s/api/email/%d?mailbox=%s", c.baseURL, id, url.QueryEscape(mailbox)),
	}
	var lastErr error
	for _, endpoint := range endpoints {
		item, err := c.fetchMailDetailEndpoint(ctx, endpoint)
		if err == nil && len(item) > 0 {
			return item, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("邮件详情为空")
}

func (c *EmailClient) fetchMailDetailEndpoint(ctx context.Context, endpoint string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("freemail detail http=%d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return parseMailDetail(body)
}

// parseMailItems 兼容三种返回：数组、{emails:[...]}、{data:[...]}。
func parseMailItems(body []byte) ([]map[string]any, error) {
	if strings.TrimSpace(string(body)) == "" {
		return nil, nil
	}
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, fmt.Errorf("freemail 返回非法 JSON: %s", truncate(string(body), 200))
	}
	for _, key := range []string{"emails", "data"} {
		if raw, ok := obj[key].([]any); ok {
			return toMapSlice(raw), nil
		}
	}
	return nil, nil
}

func parseMailDetail(body []byte) (map[string]any, error) {
	if strings.TrimSpace(string(body)) == "" {
		return nil, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		return map[string]any{"raw": string(body)}, nil
	}
	for _, key := range []string{"email", "data", "message"} {
		if nested, ok := obj[key].(map[string]any); ok {
			return nested, nil
		}
	}
	return obj, nil
}

func toMapSlice(raw []any) []map[string]any {
	out := make([]map[string]any, 0, len(raw))
	for _, v := range raw {
		if m, ok := v.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// extractEmailCode 从单封邮件中提取验证码：先看结构化字段，再正则扫描文本字段。
func extractEmailCode(item map[string]any) string {
	for _, key := range []string{"verification_code", "verificationCode", "code", "otp"} {
		if raw, ok := item[key].(string); ok && strings.TrimSpace(raw) != "" {
			return normalizeCode(raw)
		}
	}
	for _, key := range []string{
		"subject",
		"preview",
		"text",
		"text_body",
		"textBody",
		"plain",
		"body",
		"html_content",
		"htmlContent",
		"html",
		"html_body",
		"htmlBody",
		"content",
		"raw",
		"message",
	} {
		switch raw := item[key].(type) {
		case string:
			if isHTMLContentKey(key) {
				if code := extractStandaloneHTMLCode(raw); code != "" {
					return code
				}
			}
			allowLoose := key == "subject" || key == "body" || key == "html_content" || key == "htmlContent" || key == "html" || key == "html_body" || key == "htmlBody" || key == "content" || key == "raw" || key == "message"
			if code := extractEmailCodeFromText(raw, allowLoose); code != "" {
				return code
			}
		case map[string]any:
			if code := extractEmailCode(raw); code != "" {
				return code
			}
		}
	}
	return ""
}

func isHTMLContentKey(key string) bool {
	return key == "html_content" || key == "htmlContent" || key == "html" || key == "html_body" || key == "htmlBody"
}

func extractStandaloneHTMLCode(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = html.UnescapeString(text)
	text = htmlNoiseRe.ReplaceAllString(text, " ")
	text = htmlCommentRe.ReplaceAllString(text, " ")
	for _, match := range htmlTextNodeCodeRe.FindAllStringSubmatch(text, -1) {
		if len(match) < 2 {
			continue
		}
		if code := normalizeEmailCodeCandidate(match[1]); code != "" {
			return code
		}
	}
	return ""
}

func extractEmailCodeFromText(text string, allowLoose bool) string {
	for _, candidate := range normalizeEmailSearchText(text) {
		if m := emailCodeContextRe.FindStringSubmatch(candidate); m != nil {
			if code := normalizeEmailCodeCandidate(m[1]); code != "" {
				return code
			}
		}
	}
	if !allowLoose {
		return ""
	}
	for _, candidate := range normalizeEmailSearchText(text) {
		if len(candidate) > 500 {
			continue
		}
		if m := emailCodeWithDashRe.FindStringSubmatch(candidate); m != nil {
			if code := normalizeEmailCodeCandidate(m[1]); code != "" {
				return code
			}
		}
		if m := emailCodeDigitsRe.FindStringSubmatch(candidate); m != nil {
			return normalizeCode(m[1])
		}
	}
	return ""
}

func normalizeEmailCodeCandidate(raw string) string {
	code := normalizeCode(raw)
	if len(code) != 6 {
		return ""
	}
	for _, ch := range code {
		if ch >= '0' && ch <= '9' {
			return code
		}
	}
	return ""
}

func normalizeEmailSearchText(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	out := []string{}
	decoder := mime.WordDecoder{}
	if decoded, err := decoder.DecodeHeader(text); err == nil && decoded != "" && decoded != text {
		out = append(out, cleanEmailText(decoded))
	}
	unescaped := html.UnescapeString(text)
	cleaned := cleanEmailText(unescaped)
	if cleaned != "" {
		out = append(out, cleaned)
	}
	if unescaped != "" && unescaped != text {
		out = append(out, unescaped)
	}
	out = append(out, text)
	return dedupeEmailSearchTexts(out)
}

func cleanEmailText(text string) string {
	text = htmlNoiseRe.ReplaceAllString(text, " ")
	text = htmlTagRe.ReplaceAllString(text, " ")
	return strings.Join(strings.Fields(text), " ")
}

func dedupeEmailSearchTexts(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

// normalizeCode 去空白、去横杠并转大写（"abc-123" -> "ABC123"，"123456" -> "123456"）。
func normalizeCode(raw string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(raw), "-", ""))
}

// mailItemID 解析邮件 id，兼容数字与字符串两种形式。
func mailItemID(item map[string]any) int {
	switch v := item["id"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return 0
}
