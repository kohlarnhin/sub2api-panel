package register

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const sentinelFetchMaxBody = 20 << 20

type sentinelFetchCLIRequest struct {
	Proxy      string            `json:"proxy"`
	URL        string            `json:"url"`
	Method     string            `json:"method"`
	Headers    map[string]string `json:"headers"`
	BodyBase64 string            `json:"body_base64"`
}

type sentinelFetchCLIResponse struct {
	Status     int               `json:"status"`
	StatusText string            `json:"status_text"`
	Headers    map[string]string `json:"headers"`
	BodyBase64 string            `json:"body_base64"`
}

func RunSentinelFetchCLI(parent context.Context, stdin io.Reader, stdout, stderr io.Writer) int {
	var payload sentinelFetchCLIRequest
	if err := json.NewDecoder(stdin).Decode(&payload); err != nil {
		fmt.Fprintf(stderr, "decode request: %v\n", err)
		return 1
	}
	resp, err := sentinelFetch(parent, payload)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(resp); err != nil {
		fmt.Fprintf(stderr, "encode response: %v\n", err)
		return 1
	}
	return 0
}

func sentinelFetch(parent context.Context, payload sentinelFetchCLIRequest) (sentinelFetchCLIResponse, error) {
	target := strings.TrimSpace(payload.URL)
	if target == "" {
		return sentinelFetchCLIResponse{}, fmt.Errorf("missing url")
	}
	body, err := base64.StdEncoding.DecodeString(strings.TrimSpace(payload.BodyBase64))
	if err != nil {
		return sentinelFetchCLIResponse{}, fmt.Errorf("decode request body: %w", err)
	}
	method := strings.ToUpper(strings.TrimSpace(payload.Method))
	if method == "" {
		if len(body) > 0 {
			method = http.MethodPost
		} else {
			method = http.MethodGet
		}
	}
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return sentinelFetchCLIResponse{}, fmt.Errorf("create request: %w", err)
	}
	for key, value := range payload.Headers {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		switch strings.ToLower(key) {
		case "host":
			req.Host = strings.TrimSpace(value)
		case "content-length":
			continue
		default:
			req.Header.Set(key, value)
		}
	}

	client, err := newProxyHTTPClient(payload.Proxy, 45*time.Second)
	if err != nil {
		return sentinelFetchCLIResponse{}, err
	}
	httpResp, err := client.Do(req)
	if err != nil {
		return sentinelFetchCLIResponse{}, fmt.Errorf("request %s: %w", target, err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, sentinelFetchMaxBody+1))
	if err != nil {
		return sentinelFetchCLIResponse{}, fmt.Errorf("read response body: %w", err)
	}
	if len(respBody) > sentinelFetchMaxBody {
		return sentinelFetchCLIResponse{}, fmt.Errorf("response body too large")
	}

	headers := make(map[string]string, len(httpResp.Header))
	for key, values := range httpResp.Header {
		headers[strings.ToLower(key)] = strings.Join(values, ", ")
	}
	statusText := strings.TrimSpace(strings.TrimPrefix(httpResp.Status, strconv.Itoa(httpResp.StatusCode)))
	return sentinelFetchCLIResponse{
		Status:     httpResp.StatusCode,
		StatusText: statusText,
		Headers:    headers,
		BodyBase64: base64.StdEncoding.EncodeToString(respBody),
	}, nil
}
