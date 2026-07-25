package middleware

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
)

type proxyKey string

const (
	clientIPKey    proxyKey = "client_ip"
	effectiveProto proxyKey = "effective_proto"
	effectiveHost  proxyKey = "effective_host"
)

func TrustedProxy(trustedCIDRs []string) func(http.Handler) http.Handler {
	nets := make([]*net.IPNet, 0, len(trustedCIDRs))
	for _, c := range trustedCIDRs {
		_, n, err := net.ParseCIDR(strings.TrimSpace(c))
		if err == nil {
			nets = append(nets, n)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := remoteIP(r.RemoteAddr)
			proto := "http"
			if r.TLS != nil {
				proto = "https"
			}
			host := r.Host

			if trusted(ip, nets) {
				xff := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
				if net.ParseIP(xff) != nil {
					ip = xff
				}
				xfp := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0])
				if xfp == "http" || xfp == "https" {
					proto = xfp
				}
				xfh := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Host"), ",")[0])
				if validForwardedHost(xfh) {
					host = xfh
				}
			}
			ctx := context.WithValue(r.Context(), clientIPKey, ip)
			ctx = context.WithValue(ctx, effectiveProto, proto)
			ctx = context.WithValue(ctx, effectiveHost, host)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ClientIP(r *http.Request) string {
	if v, ok := r.Context().Value(clientIPKey).(string); ok && v != "" {
		return v
	}
	return remoteIP(r.RemoteAddr)
}

func EffectiveProto(r *http.Request) string {
	if v, ok := r.Context().Value(effectiveProto).(string); ok && v != "" {
		return v
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func EffectiveHost(r *http.Request) string {
	if v, ok := r.Context().Value(effectiveHost).(string); ok && v != "" {
		return v
	}
	return r.Host
}

func trusted(ip string, nets []*net.IPNet) bool {
	p := net.ParseIP(ip)
	if p == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(p) {
			return true
		}
	}
	return false
}

func remoteIP(addr string) string {
	h, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return h
}

func validForwardedHost(value string) bool {
	if value == "" || strings.Contains(value, "://") || strings.ContainsAny(value, `/\?#@`) {
		return false
	}
	if strings.IndexFunc(value, func(r rune) bool { return r <= ' ' || r == 127 }) >= 0 {
		return false
	}
	host := value
	if h, port, err := net.SplitHostPort(value); err == nil {
		host = h
		n, err := strconv.Atoi(port)
		if err != nil || n <= 0 || n > 65535 {
			return false
		}
	} else if strings.Count(value, ":") > 0 {
		return false
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return true
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
				continue
			}
			return false
		}
	}
	return true
}
