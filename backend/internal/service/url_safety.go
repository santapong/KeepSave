package service

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// ValidateWebhookURL implements ADR-0013 / audit S-H2: reject any URL that
// would have the server connect to a private network, link-local, loopback,
// cloud-metadata, or k8s service IP. Plain HTTP is rejected outside of dev.
// The check resolves the host once at validation time; a determined
// attacker could still mount a DNS-rebinding attack against the delivery
// call, but the registration check is the primary control.
func ValidateWebhookURL(rawURL string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	dev := isDevEnv()
	switch strings.ToLower(u.Scheme) {
	case "https":
		// allowed
	case "http":
		if !dev {
			return fmt.Errorf("http webhooks are only permitted when KEEPSAVE_ENV=dev")
		}
	default:
		return fmt.Errorf("unsupported url scheme %q (need https)", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("url has no host")
	}
	// Hostname-based reject: cloud metadata services often use a magic name
	// that resolves to a link-local IP; reject before DNS in case the IP
	// check is bypassed by a stale cache.
	lower := strings.ToLower(host)
	for _, bad := range []string{"metadata.google.internal", "metadata.goog", "metadata.aws.amazon.com"} {
		if lower == bad {
			return fmt.Errorf("host %q points at a cloud metadata service", host)
		}
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("resolving host %q: %w", host, err)
	}
	// In dev / test environments we still ban link-local + cloud-metadata
	// IPs (the attack we care about) but allow loopback / RFC-1918 so
	// integrators can point at httptest servers and local stacks.
	for _, ip := range ips {
		if isPrivateOrSpecial(ip, dev) {
			return fmt.Errorf("host %q resolves to a private or special-purpose IP %s", host, ip)
		}
	}
	return nil
}

// isDevEnv returns true when KEEPSAVE_ENV is unset, "dev", or
// "development" - the modes where local-loopback webhooks and plain
// HTTP are acceptable for integration testing. Any explicit non-dev
// value (production, uat, staging, ...) flips the strict deny list on.
func isDevEnv() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("KEEPSAVE_ENV")))
	return v == "" || v == "dev" || v == "development"
}

// alwaysDenyBlocks are rejected regardless of environment: link-local
// (169.254.0.0/16 covers AWS / GCP metadata) and IPv6 link-local.
var alwaysDenyBlocks = mustParseCIDRs(
	"169.254.0.0/16",
	"fe80::/10",
)

// privateBlocks are rejected in production (KEEPSAVE_ENV != "dev"). They
// are accepted in dev so integration tests can point at httptest servers
// and local backends.
var privateBlocks = mustParseCIDRs(
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"127.0.0.0/8",
	"100.64.0.0/10", // CGNAT
	"::1/128",
	"fc00::/7",
	"0.0.0.0/8",
)

func mustParseCIDRs(cidrs ...string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err == nil {
			out = append(out, n)
		}
	}
	return out
}

func isPrivateOrSpecial(ip net.IP, devMode bool) bool {
	// Metadata + link-local always blocked - even dev mode would not
	// want a webhook pointed at the EC2 metadata service.
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	for _, block := range alwaysDenyBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	if !devMode {
		if ip.IsLoopback() || ip.IsUnspecified() {
			return true
		}
		for _, block := range privateBlocks {
			if block.Contains(ip) {
				return true
			}
		}
	}
	// Optional: k8s service CIDR; operators set K8S_SERVICE_CIDR when
	// running in a cluster where the API server / kubelet are reachable
	// from the pod network. Honored in all envs (k8s metadata is
	// sensitive regardless).
	if cidr := strings.TrimSpace(os.Getenv("K8S_SERVICE_CIDR")); cidr != "" {
		if _, n, err := net.ParseCIDR(cidr); err == nil && n.Contains(ip) {
			return true
		}
	}
	return false
}
