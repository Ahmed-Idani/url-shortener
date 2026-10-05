package ip

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"
)

//get the ip address from <ip>:<port> (e.g. http.Request.RemoteAddr)
func ExtractIP(addr string) (netip.Addr, error) {
	ap, err := netip.ParseAddrPort(addr)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid address %q: %w", addr, err)
	}
	return ap.Addr(), nil
}

//get the visitor ip; behind a reverse proxy on the same machine (nginx) trust its X-Real-IP header
func ClientIP(r *http.Request) (netip.Addr, error) {
	addr, err := ExtractIP(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}, err
	}
	addr = addr.Unmap()
	if addr.IsLoopback() {
		if real, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get("X-Real-IP"))); err == nil {
			return real.Unmap(), nil
		}
	}
	return addr, nil
}