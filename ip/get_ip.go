package ip

import (
	"fmt"
	"net/netip"
)

//get the ip address from <ip>:<port> (e.g. http.Request.RemoteAddr)
func ExtractIP(addr string) (netip.Addr, error) {
	ap, err := netip.ParseAddrPort(addr)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid address %q: %w", addr, err)
	}
	return ap.Addr(), nil
}