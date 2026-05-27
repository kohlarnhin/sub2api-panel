package stats

import "strings"

// maskEmail 对邮箱地址做脱敏处理：
//   - 空字符串原样返回
//   - 不含 '@' 视作非邮箱，按用户名规则脱敏（保留首字符，其余以 *** 代替）
//   - 本地部分长度 ≤ 2：a***、ab***
//   - 本地部分长度 = 3：ab***
//   - 本地部分长度 ≥ 4：保留首 2 + 末 1，中间统一替换为 ***（与长度无关，避免泄漏长度）
//   - 域名部分保留不变
//
// 例子：
//   alice@example.com     -> al***e@example.com
//   bob@example.com       -> bo***@example.com
//   a@example.com         -> a***@example.com
//   foo.bar+x@gmail.com   -> fo***x@gmail.com
func maskEmail(s string) string {
	if s == "" {
		return ""
	}
	at := strings.LastIndex(s, "@")
	if at < 0 {
		return maskLocal(s)
	}
	local := s[:at]
	domain := s[at:]
	return maskLocal(local) + domain
}

func maskLocal(local string) string {
	switch n := len([]rune(local)); {
	case n == 0:
		return ""
	case n <= 2:
		return string([]rune(local)[0]) + "***"
	case n == 3:
		return string([]rune(local)[:2]) + "***"
	default:
		r := []rune(local)
		return string(r[:2]) + "***" + string(r[n-1:])
	}
}
