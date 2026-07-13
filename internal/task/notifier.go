package task

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

type NotificationMessage struct {
	Event      NotificationEvent `json:"event"`
	OccurredAt time.Time         `json:"occurred_at"`
	Title      string            `json:"title"`
	Message    string            `json:"message"`
	Data       map[string]any    `json:"data,omitempty"`
}

type genericWebhookPayload struct {
	Schema int `json:"schema"`
	NotificationMessage
}

type discordWebhookPayload struct {
	Username        string                 `json:"username"`
	Content         string                 `json:"content"`
	AllowedMentions discordAllowedMentions `json:"allowed_mentions"`
}

type discordAllowedMentions struct {
	Parse []string `json:"parse"`
}

var disallowedWebhookPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

type webhookNotifier struct {
	allowPrivateNetwork bool
	client              *http.Client
	resolver            *net.Resolver
}

func newWebhookNotifier(allowPrivateNetwork bool) *webhookNotifier {
	return &webhookNotifier{
		allowPrivateNetwork: allowPrivateNetwork,
		resolver:            net.DefaultResolver,
	}
}

func (notifier *webhookNotifier) Send(
	ctx context.Context,
	settings NotificationSettings,
	message NotificationMessage,
) error {
	if strings.TrimSpace(settings.WebhookURL) == "" {
		return errors.New("notification webhook URL is not configured")
	}
	if message.Event == "" {
		return errors.New("notification event is required")
	}
	if message.OccurredAt.IsZero() {
		message.OccurredAt = time.Now().UTC()
	}

	body, err := buildWebhookPayload(settings.Provider, message)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, settings.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Palworld-Server-Tool/automation")
	request.Header.Set("X-PST-Event", string(message.Event))
	if settings.Provider == NotificationGeneric && settings.Secret != "" {
		mac := hmac.New(sha256.New, []byte(settings.Secret))
		_, _ = mac.Write(body)
		request.Header.Set("X-PST-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}

	client := notifier.client
	closeIdleConnections := false
	if client == nil {
		client = notifier.safeHTTPClient(time.Duration(settings.TimeoutSeconds) * time.Second)
		closeIdleConnections = true
	}
	if closeIdleConnections {
		defer client.CloseIdleConnections()
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("webhook returned %d: %s", response.StatusCode, strings.TrimSpace(string(detail)))
	}
	return nil
}

func buildWebhookPayload(provider NotificationProvider, message NotificationMessage) ([]byte, error) {
	switch provider {
	case NotificationGeneric:
		return json.Marshal(genericWebhookPayload{Schema: 1, NotificationMessage: message})
	case NotificationDiscord:
		content := fmt.Sprintf("**%s**\n%s", strings.TrimSpace(message.Title), strings.TrimSpace(message.Message))
		content = strings.TrimSpace(content)
		if len([]rune(content)) > 1900 {
			content = string([]rune(content)[:1900])
		}
		return json.Marshal(discordWebhookPayload{
			Username:        "Palworld Server Tool",
			Content:         content,
			AllowedMentions: discordAllowedMentions{Parse: []string{}},
		})
	default:
		return nil, fmt.Errorf("unsupported notification provider %q", provider)
	}
}

func (notifier *webhookNotifier) safeHTTPClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("parse webhook address: %w", err)
		}
		if notifier.allowPrivateNetwork {
			return (&net.Dialer{Timeout: timeout}).DialContext(ctx, network, address)
		}
		if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
			return nil, errors.New("webhook host cannot use localhost")
		}
		addresses, err := notifier.resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve webhook host: %w", err)
		}
		if len(addresses) == 0 {
			return nil, errors.New("webhook host did not resolve to an IP address")
		}
		for _, resolved := range addresses {
			if isDisallowedWebhookIP(resolved.IP) {
				return nil, fmt.Errorf("webhook host resolved to a private or reserved address: %s", resolved.IP)
			}
		}
		target := net.JoinHostPort(addresses[0].IP.String(), port)
		return (&net.Dialer{Timeout: timeout}).DialContext(ctx, network, target)
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("webhook redirects are not allowed")
		},
	}
}

func isDisallowedWebhookIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() ||
		address.IsUnspecified() || address.IsLinkLocalUnicast() || address.IsMulticast() {
		return true
	}
	for _, prefix := range disallowedWebhookPrefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func webhookPreview(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host + "/…"
}
