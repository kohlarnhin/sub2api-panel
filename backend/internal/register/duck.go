package register

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const duckEmailEndpoint = "https://quack.duckduckgo.com/api/email/addresses"

var duckEmailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@duck\.com`)

type DuckClient struct {
	httpClient *http.Client
}

func NewDuckClient() *DuckClient {
	return &DuckClient{httpClient: &http.Client{Timeout: 30 * time.Second}}
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
		proxyClient, err := newProxyHTTPClient(proxyURL, 30*time.Second)
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
