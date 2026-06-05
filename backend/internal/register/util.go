package register

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func normalizePhone(phone string) string {
	value := strings.TrimSpace(phone)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "+") {
		return value
	}
	return "+" + value
}

func maskPhone(phone string) string {
	value := strings.TrimSpace(phone)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= 5 {
		return value
	}
	return string(runes[:3]) + "****" + string(runes[len(runes)-2:])
}

func stringValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case json.Number:
		return strings.TrimSpace(v.String())
	case float64:
		if math.Trunc(v) == v {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		return strconv.FormatBool(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func int64Value(value any) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return n
	default:
		return 0
	}
}

func isoToEpochSeconds(value string) int64 {
	text := strings.TrimSpace(value)
	if text == "" {
		return 0
	}
	text = strings.ReplaceAll(text, "Z", "+00:00")
	if dt, err := time.Parse(time.RFC3339, text); err == nil {
		return dt.Unix()
	}
	if dt, err := time.Parse("2006-01-02T15:04:05-07:00", text); err == nil {
		return dt.Unix()
	}
	return 0
}

func previewJSON(value map[string]any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return truncate(string(raw), 500)
}

func previewAny(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return truncate(string(raw), 500)
}

func truncate(value string, n int) string {
	if n <= 0 || len(value) <= n {
		return value
	}
	return value[:n]
}

func isLocalCallbackURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "http" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func dedupeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneInt64s(in []int64) []int64 {
	if len(in) == 0 {
		return nil
	}
	out := make([]int64, len(in))
	copy(out, in)
	return out
}
