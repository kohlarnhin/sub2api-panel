package register

import "testing"

func TestExtractEmailCodePrefersVerificationContext(t *testing.T) {
	item := map[string]any{
		"sender":            "noreply_at_tm.openai.com_elm-late-sterling@duck.com",
		"subject":           "=?UTF-8?B?5L2g55qEIE9wZW5BSSDkuLTml7bpqozor4HnoIE=?=",
		"preview":           `&#x4F60;&#x7684; OpenAI &#x4E34;&#x65F6;&#x9A8C;&#x8BC1;&#x7801; @font-face { font-family: "S&#xF6;hne"; src: url(https://example.test/123456.woff) }`,
		"verification_code": "",
		"html":              `<style>.noise{background:url("https://example.test/123456.woff")}</style><div>输入此临时验证码以继续：</div><strong>592903</strong>`,
	}

	got := extractEmailCode(item)
	if got != "592903" {
		t.Fatalf("extractEmailCode()=%q, want 592903", got)
	}
}

func TestExtractEmailCodeFromHTMLContent(t *testing.T) {
	item := map[string]any{
		"html_content": `<html><head><style>@font-face{src:url("https://example.test/123456.woff2")}</style></head><body><p>输入此临时验证码以继续：</p><p>592903</p></body></html>`,
	}

	got := extractEmailCode(item)
	if got != "592903" {
		t.Fatalf("extractEmailCode()=%q, want 592903", got)
	}
}

func TestExtractEmailCodeFromOpenAIParagraph(t *testing.T) {
	item := map[string]any{
		"html_content": `<p style="font-family: Menlo, Monaco, Lucida Console, Arial; font-size: 24px; line-height: 28px; background-color: #F3F3F3; color: #5D5D5D; border-radius: 16px; padding: 28px 24px; margin: 24px 0;">
                        <!--[if mso]>
                            <span style="font-family: Lucida Console, Arial, sans-serif;">
                        <![endif]-->
                        676199
                        <!--[if mso]>
                            </span>
                        <![endif]-->
                      </p>`,
	}

	got := extractEmailCode(item)
	if got != "676199" {
		t.Fatalf("extractEmailCode()=%q, want 676199", got)
	}
}
