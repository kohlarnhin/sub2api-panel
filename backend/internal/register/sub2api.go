package register

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	codexClientID  = "app_EMoamEEZ73f0CkXaXp7hrann"
	defaultOrgID   = "org-J8gIDzQGVzM6FABJlG4RcXkg"
	defaultPlan    = "free"
	defaultTimeout = 30 * time.Second
)

type Sub2APIClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewSub2APIClient(baseURL, apiKey string) *Sub2APIClient {
	return &Sub2APIClient{
		baseURL:    strings.TrimSpace(baseURL),
		apiKey:     strings.TrimSpace(apiKey),
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

func (c *Sub2APIClient) Upload(ctx context.Context, payload map[string]any) (map[string]any, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("Sub2API 上传内容不能为空")
	}
	baseURL, err := normalizeBaseURL(c.baseURL)
	if err != nil {
		return nil, err
	}
	if c.apiKey == "" {
		return nil, fmt.Errorf("Sub2API API Key 不能为空")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := baseURL + "/api/v1/admin/accounts"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Sub2API 上传失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var data any
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &data); err != nil {
			data = map[string]any{"text": string(respBody)}
		}
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Sub2API 上传失败: %d - %s", resp.StatusCode, previewAny(data))
	}
	return map[string]any{
		"status_code": resp.StatusCode,
		"url":         endpoint,
		"response":    data,
	}, nil
}

func BuildSub2APIPayload(email string, tokenData map[string]any, groupIDs []int64) map[string]any {
	idToken := strings.TrimSpace(stringValue(tokenData["id_token"]))
	claims := decodeJWTPayload(idToken)
	authClaims, _ := claims["https://api.openai.com/auth"].(map[string]any)
	emailValue := strings.TrimSpace(firstString(
		email,
		stringValue(tokenData["email"]),
		stringValue(claims["email"]),
	))
	expiresAt := int64Value(tokenData["expires_at"])
	if expiresAt <= 0 {
		expiresAt = isoToEpochSeconds(stringValue(tokenData["expired"]))
	}
	if expiresAt <= 0 {
		expiresAt = int64Value(claims["exp"])
	}
	if expiresAt <= 0 {
		expiresAt = time.Now().Unix()
	}
	orgID := strings.TrimSpace(firstString(
		stringValue(tokenData["organization_id"]),
		stringValue(tokenData["org_id"]),
		stringValue(authClaims["organization_id"]),
		stringValue(authClaims["org_id"]),
		defaultOrgID,
	))
	planType := normalizePlanType(firstString(stringValue(tokenData["plan_type"]), stringValue(tokenData["planType"]), defaultPlan))
	ids := normalizeGroupIDs(groupIDs)
	return map[string]any{
		"name":     emailValue,
		"platform": "openai",
		"type":     "oauth",
		"credentials": map[string]any{
			"access_token":    stringValue(tokenData["access_token"]),
			"client_id":       codexClientID,
			"email":           emailValue,
			"expires_at":      expiresAt,
			"id_token":        idToken,
			"organization_id": orgID,
			"plan_type":       planType,
			"refresh_token":   stringValue(tokenData["refresh_token"]),
		},
		"group_ids":   ids,
		"concurrency": 10,
		"priority":    1,
	}
}

func normalizeGroupIDs(values []int64) []int64 {
	out := make([]int64, 0, len(values))
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return []int64{5}
	}
	return out
}

func normalizeBaseURL(raw string) (string, error) {
	value := strings.TrimRight(strings.TrimSpace(raw), "/")
	if value == "" {
		return "", fmt.Errorf("Sub2API Base URL 不能为空")
	}
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("Sub2API Base URL 无效: %s", raw)
	}
	return value, nil
}

func decodeJWTPayload(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return map[string]any{}
	}
	raw := parts[1]
	if mod := len(raw) % 4; mod != 0 {
		raw += strings.Repeat("=", 4-mod)
	}
	data, err := base64.URLEncoding.DecodeString(raw)
	if err != nil {
		data, err = base64.RawURLEncoding.DecodeString(parts[1])
	}
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func normalizePlanType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "plus", "team", "free":
		return value
	default:
		return defaultPlan
	}
}

func firstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
