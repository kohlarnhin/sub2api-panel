package register

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const duckEmailEndpoint = "https://quack.duckduckgo.com/api/email/addresses"

var duckEmailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@duck\.com`)

type DuckClient struct {
	httpClient *http.Client
}

func NewDuckClient() *DuckClient {
	return &DuckClient{httpClient: &http.Client{Timeout: 30 * time.Second}}
}

// newDuckHTTPClient 根据代理地址创建 http.Client。
// proxy 为空时返回默认直连 Client；支持 http/https/socks5 协议。
func newDuckHTTPClient(proxyURL string) (*http.Client, error) {
	if proxyURL = strings.TrimSpace(proxyURL); proxyURL == "" {
		return &http.Client{Timeout: 30 * time.Second}, nil
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("代理地址格式错误: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "http", "https":
		transport := &http.Transport{
			Proxy:                 http.ProxyURL(parsed),
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
		return &http.Client{Timeout: 30 * time.Second, Transport: transport}, nil
	case "socks5":
		dialer, err := proxy.FromURL(parsed, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("socks5 代理初始化失败: %w", err)
		}
		contextDialer, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("socks5 代理不支持 Context 拨号")
		}
		transport := &http.Transport{
			DialContext:           contextDialer.DialContext,
			TLSClientConfig:      &tls.Config{MinVersion: tls.VersionTLS12},
			IdleConnTimeout:      90 * time.Second,
			TLSHandshakeTimeout:  10 * time.Second,
		}
		return &http.Client{Timeout: 30 * time.Second, Transport: transport}, nil
	default:
		return nil, fmt.Errorf("不支持的代理协议: %s（支持 http/https/socks5）", scheme)
	}
}

func (c *DuckClient) CreateEmail(ctx context.Context, authorization string, proxyURL string) (string, map[string]any, error) {
	authorization = normalizeDuckAuthorization(authorization)
	if authorization == "" {
		return "", nil, fmt.Errorf("Duck Authorization 不能为空")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, duckEmailEndpoint, bytes.NewReader(nil))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("authorization", authorization)
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("content-length", "0")
	req.Header.Set("origin", "https://duckduckgo.com")
	req.Header.Set("pragma", "no-cache")
	req.Header.Set("referer", "https://duckduckgo.com/")
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-site")
	req.Header.Set("sec-gpc", "1")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	// 根据代理配置创建 HTTP Client
	client := c.httpClient
	if proxyURL != "" {
		proxyClient, err := newDuckHTTPClient(proxyURL)
		if err != nil {
			return "", nil, err
		}
		client = proxyClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}
	data := map[string]any{}
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &data); err != nil {
			data = map[string]any{"raw": strings.TrimSpace(string(body))}
		}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", data, fmt.Errorf("Duck 邮箱创建失败: %d - %s", resp.StatusCode, truncate(string(body), 300))
	}
	email := extractDuckEmail(data)
	if email == "" {
		return "", data, fmt.Errorf("Duck 返回缺少邮箱地址: %s", previewAny(data))
	}
	return email, data, nil
}

func normalizeDuckAuthorization(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(value), "bearer ") {
		return value
	}
	return "Bearer " + value
}

func extractDuckEmail(data map[string]any) string {
	for _, key := range []string{"email", "address", "email_address"} {
		if email := normalizeDuckEmail(stringValue(data[key])); email != "" {
			return email
		}
	}
	if email := findDuckEmail(data); email != "" {
		return email
	}
	return ""
}

func findDuckEmail(value any) string {
	switch v := value.(type) {
	case map[string]any:
		for _, item := range v {
			if email := findDuckEmail(item); email != "" {
				return email
			}
		}
	case []any:
		for _, item := range v {
			if email := findDuckEmail(item); email != "" {
				return email
			}
		}
	case string:
		if match := duckEmailPattern.FindString(v); match != "" {
			return strings.ToLower(match)
		}
		return normalizeDuckEmail(v)
	}
	return ""
}

func normalizeDuckEmail(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	if strings.Contains(value, "@") {
		if strings.HasSuffix(value, "@duck.com") {
			return value
		}
		return ""
	}
	if strings.ContainsAny(value, " /\\?&#") {
		return ""
	}
	return value + "@duck.com"
}
