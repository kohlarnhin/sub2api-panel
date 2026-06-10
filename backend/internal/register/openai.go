package register

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	stdhttp "net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/fhttp/cookiejar"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

const (
	authURL            = "https://auth.openai.com/oauth/authorize"
	tokenURL           = "https://auth.openai.com/oauth/token"
	defaultRedirectURI = "http://localhost:1455/auth/callback"
	defaultScope       = "openid email profile offline_access"

	chatGPTAuthURL     = "https://auth.openai.com/api/accounts/authorize"
	chatGPTClientID    = "app_X8zY6vW2pQ9tR3dE7nK1jL5gH"
	chatGPTRedirectURI = "https://chatgpt.com/api/auth/callback/openai"
	chatGPTScope       = "openid email profile offline_access model.request model.read organization.read organization.write"
	chatGPTAudience    = "https://api.openai.com/v1"
	createAccountFlow  = "oauth_create_account"
	browserAcceptLang  = "zh-CN,zh;q=0.9"
	browserUserAgent   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"
	browserSecCHUA     = `"Chromium";v="146", "Not-A.Brand";v="24", "Google Chrome";v="146"`
	browserSecCHUAPlat = `"Windows"`
	sentinelMaxRetries = 3
	defaultHTTPTimeout = 30 * time.Second
)

type AuthSession struct {
	Client       tls_client.HttpClient
	Jar          http.CookieJar
	OAuthState   string
	CodeVerifier string
	AuthURL      string
}

type sentinelInput struct {
	DID          string `json:"did"`
	Flow         string `json:"flow"`
	PageURL      string `json:"page_url"`
	UserAgent    string `json:"user_agent"`
	CookieHeader string `json:"cookie_header"`
	Proxy        string `json:"proxy"`
	FetchHelper  string `json:"fetch_helper,omitempty"`
}

// newImpersonatedClient 创建一个伪装成 Chrome TLS 指纹的 HTTP 客户端，
// 用于通过 auth.openai.com 前置的 Cloudflare Bot 检测（等价于参考项目里
// curl_cffi 的 impersonate="chrome"）。传入 jar 时共享同一 cookie 容器。
func newImpersonatedClient(jar http.CookieJar, proxyURL string) (tls_client.HttpClient, error) {
	opts := []tls_client.HttpClientOption{
		tls_client.WithClientProfile(profiles.Chrome_146_PSK),
		tls_client.WithTimeoutSeconds(int(defaultHTTPTimeout / time.Second)),
		tls_client.WithDefaultHeaders(defaultBrowserHeaders()),
	}
	if jar != nil {
		opts = append(opts, tls_client.WithCookieJar(jar))
	}
	if proxyURL = strings.TrimSpace(proxyURL); proxyURL != "" {
		opts = append(opts, tls_client.WithProxyUrl(proxyURL))
	}
	return tls_client.NewHttpClient(tls_client.NewNoopLogger(), opts...)
}

func newAuthSession() (*AuthSession, error) {
	return newAuthSessionWithProxy("")
}

func newAuthSessionWithProxy(proxyURL string) (*AuthSession, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	client, err := newImpersonatedClient(jar, proxyURL)
	if err != nil {
		return nil, err
	}
	client.SetFollowRedirect(true)
	return &AuthSession{
		Client: client,
		Jar:    jar,
	}, nil
}

func generateCodexOAuthURL() (string, string, string, error) {
	state, err := randomURLToken(16)
	if err != nil {
		return "", "", "", err
	}
	verifier, err := randomURLToken(64)
	if err != nil {
		return "", "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	values := url.Values{}
	values.Set("client_id", codexClientID)
	values.Set("response_type", "code")
	values.Set("redirect_uri", defaultRedirectURI)
	values.Set("scope", defaultScope)
	values.Set("state", state)
	values.Set("code_challenge", challenge)
	values.Set("code_challenge_method", "S256")
	values.Set("prompt", "login")
	values.Set("id_token_add_organizations", "true")
	values.Set("codex_cli_simplified_flow", "true")
	return authURL + "?" + values.Encode(), state, verifier, nil
}

func generateChatGPTOAuthURL() (string, string, error) {
	state, err := randomURLToken(16)
	if err != nil {
		return "", "", err
	}
	deviceID := newID()
	values := url.Values{}
	values.Set("client_id", chatGPTClientID)
	values.Set("scope", chatGPTScope)
	values.Set("response_type", "code")
	values.Set("redirect_uri", chatGPTRedirectURI)
	values.Set("audience", chatGPTAudience)
	values.Set("device_id", deviceID)
	values.Set("prompt", "login")
	values.Set("ext-oai-did", deviceID)
	values.Set("ext-passkey-client-capabilities", "1111")
	values.Set("screen_hint", "login_or_signup")
	values.Set("state", state)
	return chatGPTAuthURL + "?" + values.Encode(), state, nil
}

func baseHeaders(referer string) http.Header {
	h := http.Header{}
	h.Set("referer", referer)
	h.Set("origin", "https://auth.openai.com")
	h.Set("accept", "application/json")
	h.Set("content-type", "application/json")
	h.Set("accept-language", browserAcceptLang)
	h.Set("cache-control", "no-cache")
	h.Set("pragma", "no-cache")
	h.Set("sec-ch-ua", browserSecCHUA)
	h.Set("sec-ch-ua-mobile", "?0")
	h.Set("sec-ch-ua-platform", browserSecCHUAPlat)
	h.Set("sec-fetch-dest", "empty")
	h.Set("sec-fetch-mode", "cors")
	h.Set("sec-fetch-site", "same-origin")
	h.Set("user-agent", browserUserAgent)
	return h
}

// applyBrowserHeaders 复刻参考项目 curl_cffi session 级默认头（UA + 客户端提示），
// 补到那些只设了少量头的请求上。tls-client 仅在请求「未设置任何头」时才套用默认头
// （client.go: if len(req.Header)==0），不会与请求头合并，因此必须显式补齐，
// 否则像 phone-otp/send 这类只带 referer/accept 的请求会因缺 UA 被 Cloudflare 拦成 403。
func applyBrowserHeaders(h http.Header) {
	h.Set("user-agent", browserUserAgent)
	h.Set("accept-language", browserAcceptLang)
	h.Set("sec-ch-ua", browserSecCHUA)
	h.Set("sec-ch-ua-mobile", "?0")
	h.Set("sec-ch-ua-platform", browserSecCHUAPlat)
}

func defaultBrowserHeaders() http.Header {
	h := http.Header{}
	h.Set("accept", "*/*")
	applyBrowserHeaders(h)
	return h
}

func authCookieHeader(jar http.CookieJar) string {
	return authCookieHeaderForURL(jar, "https://auth.openai.com/")
}

func authCookieHeaderForURL(jar http.CookieJar, rawURL string) string {
	if jar == nil {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return cookieHeaderFromCookies(jar.Cookies(parsed))
}

func cookieHeaderFromCookies(cookies []*http.Cookie) string {
	var parts []string
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		parts = append(parts, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(parts, "; ")
}

func cookieValue(jar http.CookieJar, rawURL string, name string) string {
	if jar == nil {
		return ""
	}
	parsed, _ := url.Parse(rawURL)
	for _, cookie := range jar.Cookies(parsed) {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func authSessionCookieValue(jar http.CookieJar) string {
	for _, rawURL := range []string{
		"https://auth.openai.com/",
		"https://auth.openai.com/api/accounts/workspace/select",
		"https://auth.openai.com/api/accounts/organization/select",
		"https://auth.openai.com/api/accounts/email-otp/validate",
		"https://auth.openai.com/api/accounts/add-email/send",
		"https://auth.openai.com/api/oauth/oauth2/auth",
		"https://auth.openai.com/sign-in-with-chatgpt/codex/consent",
		"https://auth.openai.com/contact-verification",
		"https://auth.openai.com/email-verification",
	} {
		if value := cookieValue(jar, rawURL, "oai-client-auth-session"); value != "" {
			return value
		}
	}
	return ""
}

func mintSentinelHeaders(ctx context.Context, scriptPath, did, flow, cookieHeader, pageURL, proxyURL string) (http.Header, error) {
	if strings.TrimSpace(did) == "" {
		return nil, fmt.Errorf("生成 Sentinel 头时缺少 oai-did")
	}
	if pageURL == "" {
		if flow == createAccountFlow {
			pageURL = "https://auth.openai.com/about-you"
		} else {
			pageURL = "https://auth.openai.com/create-account"
		}
	}
	input := sentinelInput{
		DID:          did,
		Flow:         flow,
		PageURL:      pageURL,
		UserAgent:    browserUserAgent,
		CookieHeader: cookieHeader,
		Proxy:        strings.TrimSpace(proxyURL),
		FetchHelper:  sentinelFetchHelperPath(),
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 0; attempt < sentinelMaxRetries; attempt++ {
		cmdCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		cmd := exec.CommandContext(cmdCtx, "node", scriptPath)
		cmd.Stdin = bytes.NewReader(payload)
		if input.FetchHelper != "" {
			cmd.Env = append(os.Environ(), "SUB2API_SENTINEL_FETCH_HELPER="+input.FetchHelper)
		}
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			detail := strings.TrimSpace(string(out))
			lastErr = fmt.Errorf("Sentinel helper 执行失败: %s", truncate(detail, 300))
			if strings.Contains(detail, "node_watchdog") && attempt < sentinelMaxRetries-1 {
				time.Sleep(time.Duration(5*(attempt+1)) * time.Second)
				continue
			}
			return nil, lastErr
		}
		var result struct {
			Headers map[string]string `json:"headers"`
		}
		if err := json.Unmarshal(bytes.TrimSpace(out), &result); err != nil {
			return nil, fmt.Errorf("Sentinel helper 输出不是合法 JSON: %s", truncate(string(out), 200))
		}
		token := strings.TrimSpace(result.Headers["OpenAI-Sentinel-Token"])
		if token == "" {
			return nil, fmt.Errorf("Sentinel helper 未返回 OpenAI-Sentinel-Token")
		}
		headers := http.Header{}
		headers.Set("openai-sentinel-token", token)
		if so := strings.TrimSpace(result.Headers["OpenAI-Sentinel-SO-Token"]); so != "" {
			headers.Set("openai-sentinel-so-token", so)
		}
		return headers, nil
	}
	return nil, lastErr
}

func sentinelFetchHelperPath() string {
	if value := strings.TrimSpace(os.Getenv("SUB2API_SENTINEL_FETCH_HELPER")); value != "" {
		return value
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}

func exchangeToken(ctx context.Context, code, verifier string) (map[string]any, error) {
	return exchangeTokenWithProxy(ctx, code, verifier, "")
}

func exchangeTokenWithProxy(ctx context.Context, code, verifier string, proxyURL string) (map[string]any, error) {
	body := url.Values{}
	body.Set("grant_type", "authorization_code")
	body.Set("client_id", codexClientID)
	body.Set("code", code)
	body.Set("redirect_uri", defaultRedirectURI)
	body.Set("code_verifier", verifier)
	bodyText := body.Encode()
	req, err := stdhttp.NewRequestWithContext(ctx, stdhttp.MethodPost, tokenURL, strings.NewReader(bodyText))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	client := stdhttp.DefaultClient
	if strings.TrimSpace(proxyURL) != "" {
		proxyClient, err := newProxyHTTPClient(proxyURL, defaultHTTPTimeout)
		if err != nil {
			return nil, err
		}
		client = proxyClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if len(respBody) > 0 {
		_ = json.Unmarshal(respBody, &data)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("token exchange failed: %d: %s", resp.StatusCode, truncate(string(respBody), 500))
	}
	if data == nil {
		data = map[string]any{}
	}
	return data, nil
}

func scriptPathFromConfig(configPath string) string {
	if configPath != "" {
		dir := filepath.Dir(configPath)
		candidate := filepath.Join(dir, "scripts", "openai_sentinel_headers.js")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "scripts", "openai_sentinel_headers.js")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	candidate := filepath.Join("scripts", "openai_sentinel_headers.js")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return candidate
}

func randomURLToken(byteLen int) (string, error) {
	raw := make([]byte, byteLen)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func randomPassword(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var b strings.Builder
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			b.WriteByte(chars[time.Now().UnixNano()%int64(len(chars))])
			continue
		}
		b.WriteByte(chars[n.Int64()])
	}
	return b.String()
}

func newID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:])
}
