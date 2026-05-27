package stats

import "testing"

func TestMaskEmail(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"single char local", "a@example.com", "a***@example.com"},
		{"two char local", "ab@example.com", "a***@example.com"},
		{"three char local", "bob@example.com", "bo***@example.com"},
		{"four char local", "alice@example.com", "al***e@example.com"},
		{"long local with dots", "foo.bar+x@gmail.com", "fo***x@gmail.com"},
		{"unicode local", "张三@example.com", "张***@example.com"},
		{"no at sign", "plainuser", "pl***r"},
		{"multiple at signs uses last", "a@b@example.com", "a@***@example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := maskEmail(tc.in)
			if got != tc.want {
				t.Fatalf("maskEmail(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
