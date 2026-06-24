package register

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const heroSMSBaseURL = "https://hero-sms.com/stubs/handler_api.php"

type HeroSMSClient struct {
	httpClient *http.Client
}

type HeroSMSError struct {
	Message    string
	StatusCode int
	Payload    map[string]any
}

func (e *HeroSMSError) Error() string {
	return e.Message
}

func NewHeroSMSClient() *HeroSMSClient {
	return &HeroSMSClient{httpClient: &http.Client{Timeout: 30 * time.Second}}
}

func (c *HeroSMSClient) clientForProxy(proxyURL string) (*http.Client, error) {
	if strings.TrimSpace(proxyURL) == "" {
		return c.httpClient, nil
	}
	return newProxyHTTPClient(proxyURL, 30*time.Second)
}

func DefaultTemplate() HeroSMSTemplate {
	return HeroSMSTemplate{
		Name:           DefaultHeroSMSTemplateName,
		Service:        DefaultHeroSMSService,
		Country:        DefaultHeroSMSCountry,
		Operator:       DefaultHeroSMSOperator,
		MaxPrice:       DefaultHeroSMSMaxPrice,
		FixedPrice:     false,
		Owner:          DefaultHeroSMSOwner,
		ActivationType: DefaultHeroSMSActivation,
		Amount:         DefaultHeroSMSAmount,
		Enabled:        true,
		SortOrder:      0,
	}
}

func normalizeHeroSMSTemplate(template HeroSMSTemplate) HeroSMSTemplate {
	template.Name = strings.TrimSpace(template.Name)
	if template.Name == "" {
		template.Name = DefaultHeroSMSTemplateName
	}
	template.Service = strings.TrimSpace(template.Service)
	if template.Service == "" {
		template.Service = DefaultHeroSMSService
	}
	if template.Country <= 0 {
		template.Country = DefaultHeroSMSCountry
	}
	template.Operator = strings.TrimSpace(template.Operator)
	if template.Operator == "" {
		template.Operator = DefaultHeroSMSOperator
	}
	if template.MaxPrice <= 0 {
		template.MaxPrice = DefaultHeroSMSMaxPrice
	}
	if template.Owner <= 0 {
		template.Owner = DefaultHeroSMSOwner
	}
	if template.ActivationType < 0 {
		template.ActivationType = DefaultHeroSMSActivation
	}
	if template.Amount <= 0 {
		template.Amount = DefaultHeroSMSAmount
	}
	return template
}

func (c *HeroSMSClient) GetNumber(ctx context.Context, apiKey string, template HeroSMSTemplate, proxyURL string) (map[string]any, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("HeroSMS api_key 不能为空")
	}
	template = normalizeHeroSMSTemplate(template)

	params := url.Values{}
	params.Set("action", "getNumberV2")
	params.Set("api_key", apiKey)
	params.Set("service", template.Service)
	params.Set("country", strconv.Itoa(template.Country))
	params.Set("operator", template.Operator)
	params.Set("maxPrice", trimFloat(template.MaxPrice))
	params.Set("fixedPrice", strconv.FormatBool(template.FixedPrice))

	reqURL := heroSMSBaseURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	client, err := c.clientForProxy(proxyURL)
	if err != nil {
		return nil, err
	}
	result, statusCode, err := doJSON(client, req)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, &HeroSMSError{
			Message:    fmt.Sprintf("HeroSMS 获取手机号失败: %d - %s", statusCode, previewJSON(result)),
			StatusCode: statusCode,
			Payload:    result,
		}
	}
	phone := normalizePhone(stringValue(result["phoneNumber"]))
	if phone != "" {
		result["phone_number"] = phone
	}
	result["provider"] = "herosms"
	result["request"] = map[string]any{
		"service":        template.Service,
		"country":        template.Country,
		"operator":       template.Operator,
		"maxPrice":       template.MaxPrice,
		"fixedPrice":     template.FixedPrice,
		"owner":          template.Owner,
		"activationType": template.ActivationType,
		"amount":         template.Amount,
	}
	return result, nil
}

func (c *HeroSMSClient) GetStatus(ctx context.Context, apiKey string, activationID string, proxyURL string) (map[string]any, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("HeroSMS api_key 不能为空")
	}
	activationID = strings.TrimSpace(activationID)
	if activationID == "" {
		return nil, fmt.Errorf("HeroSMS 激活 ID 不能为空")
	}
	id, err := strconv.Atoi(activationID)
	if err != nil {
		return nil, fmt.Errorf("HeroSMS 激活 ID 必须是整数: %w", err)
	}
	params := url.Values{}
	params.Set("action", "getStatusV2")
	params.Set("api_key", apiKey)
	params.Set("id", strconv.Itoa(id))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, heroSMSBaseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	client, err := c.clientForProxy(proxyURL)
	if err != nil {
		return nil, err
	}
	result, statusCode, err := doJSON(client, req)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, &HeroSMSError{
			Message:    fmt.Sprintf("HeroSMS 获取激活状态失败: %d - %s", statusCode, previewJSON(result)),
			StatusCode: statusCode,
			Payload:    result,
		}
	}
	result["provider"] = "herosms"
	result["activation_id"] = id
	result["verification_code"] = ExtractHeroSMSCode(result)
	return result, nil
}

func (c *HeroSMSClient) SetStatus(ctx context.Context, apiKey string, activationID string, status int, proxyURL string) (map[string]any, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("HeroSMS api_key 不能为空")
	}
	activationID = strings.TrimSpace(activationID)
	if activationID == "" {
		return nil, fmt.Errorf("HeroSMS 激活 ID 不能为空")
	}
	if status != 3 && status != 6 && status != 8 {
		return nil, fmt.Errorf("HeroSMS 激活状态只支持 3、6、8")
	}
	id, err := strconv.Atoi(activationID)
	if err != nil {
		return nil, fmt.Errorf("HeroSMS 激活 ID 必须是整数: %w", err)
	}
	params := url.Values{}
	params.Set("action", "setStatus")
	params.Set("api_key", apiKey)
	params.Set("id", strconv.Itoa(id))
	params.Set("status", strconv.Itoa(status))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, heroSMSBaseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	client, err := c.clientForProxy(proxyURL)
	if err != nil {
		return nil, err
	}
	result, statusCode, err := doJSON(client, req)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, &HeroSMSError{
			Message:    fmt.Sprintf("HeroSMS 更改激活状态失败: %d - %s", statusCode, previewJSON(result)),
			StatusCode: statusCode,
			Payload:    result,
		}
	}
	result["provider"] = "herosms"
	result["activation_id"] = id
	result["activation_status"] = status
	return result, nil
}

func ExtractHeroSMSCode(statusData map[string]any) string {
	for _, channel := range []string{"sms", "call"} {
		payload, _ := statusData[channel].(map[string]any)
		code := strings.TrimSpace(stringValue(payload["code"]))
		if code != "" && strings.ToLower(code) != "code" {
			return code
		}
	}
	raw := stringValue(statusData["raw"])
	if raw == "" {
		return ""
	}
	re := regexp.MustCompile(`\b(\d{4,8})\b`)
	match := re.FindStringSubmatch(raw)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func IsHeroSMSNoNumbersError(err error) bool {
	heroErr, ok := err.(*HeroSMSError)
	if !ok || heroErr.StatusCode != http.StatusNotFound {
		return false
	}
	title := strings.ToUpper(strings.TrimSpace(stringValue(heroErr.Payload["title"])))
	details := stringValue(heroErr.Payload["details"])
	text := err.Error()
	return title == "NO_NUMBERS" || strings.Contains(text, "NO_NUMBERS") || strings.Contains(details, "Numbers Not Found")
}

func doJSON(client *http.Client, req *http.Request) (map[string]any, int, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	var data any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &data); err != nil {
			data = map[string]any{"raw": strings.TrimSpace(string(body))}
		}
	}
	if data == nil {
		data = map[string]any{}
	}
	if obj, ok := data.(map[string]any); ok {
		return obj, resp.StatusCode, nil
	}
	return map[string]any{"data": data}, resp.StatusCode, nil
}

func trimFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
