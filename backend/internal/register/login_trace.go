package register

import (
	"encoding/json"
	"fmt"
)

func (s *Service) startLoginTrace(sessionID string, task userLoginTask) string {
	return ""
}

func (s *Service) traceLogin(sessionID, event string, fields map[string]any) {
}

func traceHeaders(headers map[string][]string) map[string][]string {
	out := make(map[string][]string, len(headers))
	for key, values := range headers {
		out[key] = append([]string(nil), values...)
	}
	return out
}

func traceError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func traceJSON(value any) any {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return string(raw)
	}
	return out
}
