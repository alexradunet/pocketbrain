package web

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// IsLocalOnlyAddr reports whether addr binds only to loopback/localhost.
func IsLocalOnlyAddr(addr string) bool {
	host := addr
	if strings.Contains(addr, ":") {
		h, _, err := net.SplitHostPort(addr)
		if err == nil {
			host = h
		}
	}

	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}

	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func websocketCheckOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}

	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	originHost := strings.TrimSpace(u.Hostname())
	if originHost == "" {
		return false
	}

	reqHost := strings.TrimSpace(r.Host)
	if reqHost == "" {
		return false
	}
	if h, _, err := net.SplitHostPort(reqHost); err == nil {
		reqHost = h
	}

	if strings.EqualFold(originHost, reqHost) {
		return true
	}

	originIP := net.ParseIP(originHost)
	reqIP := net.ParseIP(reqHost)
	return originIP != nil && reqIP != nil && originIP.IsLoopback() && reqIP.IsLoopback()
}
