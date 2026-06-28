package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var errUnsafeOutboundURL = errors.New("unsafe outbound url")

func NewHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return ValidateOutboundURL(req.Context(), req.URL.String())
		},
	}
}

func ValidateOutboundURL(ctx context.Context, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: parse url: %v", errUnsafeOutboundURL, err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("%w: unsupported scheme %q", errUnsafeOutboundURL, u.Scheme)
	}
	if u.User != nil {
		return fmt.Errorf("%w: userinfo is not allowed", errUnsafeOutboundURL)
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return fmt.Errorf("%w: host is required", errUnsafeOutboundURL)
	}
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("%w: localhost is not allowed", errUnsafeOutboundURL)
	}

	ips := []net.IP{}
	if ip := net.ParseIP(host); ip != nil {
		ips = append(ips, ip)
	} else {
		lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		addrs, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
		if err != nil {
			return fmt.Errorf("%w: resolve host: %v", errUnsafeOutboundURL, err)
		}
		for _, addr := range addrs {
			ips = append(ips, addr.IP)
		}
	}
	if len(ips) == 0 {
		return fmt.Errorf("%w: host has no addresses", errUnsafeOutboundURL)
	}
	for _, ip := range ips {
		if isUnsafeIP(ip) {
			return fmt.Errorf("%w: host resolves to private address", errUnsafeOutboundURL)
		}
	}
	return nil
}

func isUnsafeIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsUnspecified() ||
		ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast()
}
