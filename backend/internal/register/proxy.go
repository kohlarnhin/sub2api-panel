package register

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

func newProxyHTTPClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return &http.Client{Timeout: timeout}, nil
	}
	transport, err := newProxyTransport(proxyURL)
	if err != nil {
		return nil, err
	}
	return &http.Client{Timeout: timeout, Transport: transport}, nil
}

func newProxyTransport(proxyURL string) (*http.Transport, error) {
	parsed, err := url.Parse(strings.TrimSpace(proxyURL))
	if err != nil {
		return nil, fmt.Errorf("代理地址格式错误: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	transport := &http.Transport{
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	switch scheme {
	case "http", "https":
		transport.Proxy = http.ProxyURL(parsed)
	case "socks5":
		dialer, err := proxy.FromURL(parsed, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("socks5 代理初始化失败: %w", err)
		}
		contextDialer, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("socks5 代理不支持 Context 拨号")
		}
		transport.DialContext = contextDialer.DialContext
	default:
		return nil, fmt.Errorf("不支持的代理协议: %s（支持 http/https/socks5）", scheme)
	}
	return transport, nil
}

func validateProxyURL(proxyURL string) error {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return nil
	}
	transport, err := newProxyTransport(proxyURL)
	if err != nil {
		return err
	}
	transport.CloseIdleConnections()
	return nil
}

type proxyConfigSnapshot struct {
	ProxyURL     string
	SMSProxy     bool
	OpenAIProxy  bool
	EmailProxy   bool
	Sub2APIProxy bool
}

func (p proxyConfigSnapshot) forSMS() string {
	if p.SMSProxy {
		return strings.TrimSpace(p.ProxyURL)
	}
	return ""
}

func (p proxyConfigSnapshot) forOpenAI() string {
	if p.OpenAIProxy {
		return strings.TrimSpace(p.ProxyURL)
	}
	return ""
}

func (p proxyConfigSnapshot) forEmail() string {
	if p.EmailProxy {
		return strings.TrimSpace(p.ProxyURL)
	}
	return ""
}

func (p proxyConfigSnapshot) forSub2API() string {
	if p.Sub2APIProxy {
		return strings.TrimSpace(p.ProxyURL)
	}
	return ""
}
