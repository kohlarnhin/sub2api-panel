# OpenAI Codex Auth Cookie Jar Pitfall

## Background

The phone registration login flow signs in to `auth.openai.com`, binds the
assigned email, completes Codex consent/workspace selection, and exchanges the
localhost OAuth callback code for tokens.

The reference Python implementation uses a browser-like session. The Go
implementation uses `tls-client` for Chrome TLS impersonation.

## Pitfall

Do not use `tls_client.NewCookieJar()` for the OpenAI auth session.

In `github.com/bogdanfinn/tls-client@v1.14.0`, the custom cookie jar keeps its
own `allCookies` map and does not reliably apply `Expires=Thu, 01 Jan 1970...`
deletion semantics. During the Codex consent/workspace flow, OpenAI sends
deletion cookies such as:

- `login_session=null`
- `auth_provider=null`
- `unified_session_manifest=null`
- `auth-session-minimized=null`

A browser would remove those cookies. The `tls-client` jar can keep sending
them, which corrupts the app session state. The OAuth callback URL can still be
generated, but the later token exchange fails with:

```text
token exchange failed: 500 ... code: missing_existing_app_session
```

## Required Pattern

Use the standard `fhttp/cookiejar` with `tls-client`:

```go
jar, err := cookiejar.New(nil)
client, err := newImpersonatedClient(jar)
```

Keep the Chrome TLS client profile, but pass the normal jar through
`tls_client.WithCookieJar(jar)`. This preserves browser-like cookie expiry and
deletion behavior while keeping the impersonated transport.

When logging or passing cookies to Sentinel, derive the cookie header for the
actual request URL instead of merging cookies from many auth URLs:

```go
authCookieHeaderForURL(auth.Jar, "https://auth.openai.com/log-in")
```

## Debugging Notes

If this fails again, first check the request before the final
`/api/oauth/oauth2/auth?...consent_verifier=...` redirect. It should not contain
deleted cookie values like `login_session=null` or `auth_provider=null`.

The `/oauth/token` request itself is intentionally simple form data and does not
need the auth cookies. `missing_existing_app_session` usually means the callback
code was created from a broken or unfinished app session earlier in the flow.
