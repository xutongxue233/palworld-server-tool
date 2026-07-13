package fleet

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var disallowedPublicPrefixes = []netip.Prefix{
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

func NewHTTPClient(node NodeConfig, requestTimeout time.Duration) *http.Client {
	if requestTimeout <= 0 {
		requestTimeout = node.Timeout
	}
	if requestTimeout <= 0 {
		requestTimeout = defaultTimeoutSeconds * time.Second
	}
	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: requestTimeout,
		IdleConnTimeout:       30 * time.Second,
		MaxIdleConnsPerHost:   4,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	transport.DialContext = safeDialContext(node.AllowPrivateNetwork, node.Timeout)
	return &http.Client{
		Transport: transport,
		Timeout:   requestTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("fleet node redirects are not allowed")
		},
	}
}

func BuildNodeURL(node NodeConfig, targetPath, rawQuery string) (string, error) {
	if !strings.HasPrefix(targetPath, "/api/") || strings.Contains(targetPath, "\\") {
		return "", errors.New("fleet target must be an absolute /api path")
	}
	parsed := node.parsedURL
	if parsed == nil {
		var err error
		parsed, _, err = parseNodeBaseURL(node.BaseURL)
		if err != nil {
			return "", err
		}
	}
	copyURL := *parsed
	copyURL.Path = targetPath
	copyURL.RawPath = ""
	copyURL.RawQuery = rawQuery
	return copyURL.String(), nil
}

func safeDialContext(allowPrivate bool, timeout time.Duration) func(context.Context, string, string) (net.Conn, error) {
	if timeout <= 0 {
		timeout = defaultTimeoutSeconds * time.Second
	}
	resolver := net.DefaultResolver
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		dialContext, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("parse fleet node address: %w", err)
		}
		if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
			if !allowPrivate {
				return nil, errors.New("fleet node host cannot use localhost without allow_private_network")
			}
		}
		addresses, err := resolver.LookupIPAddr(dialContext, host)
		if err != nil {
			return nil, fmt.Errorf("resolve fleet node host: %w", err)
		}
		if len(addresses) == 0 {
			return nil, errors.New("fleet node host did not resolve to an IP address")
		}
		for _, resolved := range addresses {
			if forbiddenFleetIP(resolved.IP, allowPrivate) {
				return nil, fmt.Errorf("fleet node host resolved to a disallowed address: %s", resolved.IP)
			}
		}
		return dialFleetAddresses(dialContext, dialer, network, port, addresses)
	}
}

func dialFleetAddresses(
	ctx context.Context,
	dialer *net.Dialer,
	network,
	port string,
	addresses []net.IPAddr,
) (net.Conn, error) {
	dialErrors := make([]error, 0, len(addresses))
	for _, resolved := range addresses {
		target := net.JoinHostPort(resolved.IP.String(), port)
		connection, err := dialer.DialContext(ctx, network, target)
		if err == nil {
			return connection, nil
		}
		dialErrors = append(dialErrors, fmt.Errorf("%s: %w", target, err))
		if ctx.Err() != nil {
			break
		}
	}
	if len(dialErrors) == 0 {
		return nil, errors.New("fleet node host did not provide a dialable IP address")
	}
	return nil, fmt.Errorf("dial fleet node: %w", errors.Join(dialErrors...))
}

func forbiddenFleetIP(ip net.IP, allowPrivate bool) bool {
	if ip == nil {
		return true
	}
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	address = address.Unmap()
	if address.IsUnspecified() || address.IsMulticast() {
		return true
	}
	if allowPrivate {
		return false
	}
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() ||
		address.IsLinkLocalUnicast() {
		return true
	}
	for _, prefix := range disallowedPublicPrefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func ManagementURL(node NodeConfig) string {
	parsed, err := url.Parse(node.BaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}
