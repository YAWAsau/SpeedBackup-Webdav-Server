package sbserver

import (
	"net"
	"net/http"
	"sort"
)

type davIPv4 struct {
	IPv4      string `json:"ipv4"`
	Interface string `json:"interface"`
	URL       string `json:"url"`
	index     int
}

func interfaceIPv4() ([]davIPv4, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	result := []davIPv4{}
	seen := map[string]bool{}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil || ip.To4() == nil || !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			value := ip.String()
			if !seen[value] {
				seen[value] = true
				result = append(result, davIPv4{IPv4: value, Interface: iface.Name, index: iface.Index})
			}
		}
	}
	return result, nil
}

// UDP connect asks the OS which local address its current route would use.
// No Read/Write is performed: this sends no packet and requires no DNS/server.
func routedIPv4() string {
	c, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 9})
	if err != nil {
		return ""
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).IP.String()
}

func connectionIPv4(list []davIPv4, listen, local, route, scheme string) ([]davIPv4, bool) {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return []davIPv4{}, false
	}
	localHost, localPort, _ := net.SplitHostPort(local)
	if localPort != "" {
		port = localPort
	}
	bound := net.ParseIP(host)
	loopback := bound != nil && bound.IsLoopback() || host == "localhost"
	if loopback || port == "" {
		return []davIPv4{}, loopback
	}
	result := []davIPv4{}
	for _, a := range list {
		if host != "" && (bound == nil || !bound.IsUnspecified() && !bound.Equal(net.ParseIP(a.IPv4))) {
			continue
		}
		a.URL = scheme + "://" + net.JoinHostPort(a.IPv4, port) + "/"
		result = append(result, a)
	}
	rank := func(a davIPv4) int {
		if a.IPv4 == localHost {
			return 0
		}
		if a.IPv4 == route {
			return 1
		}
		if net.ParseIP(a.IPv4).IsPrivate() {
			return 2
		}
		return 3
	}
	sort.SliceStable(result, func(i, j int) bool {
		if rank(result[i]) != rank(result[j]) {
			return rank(result[i]) < rank(result[j])
		}
		if result[i].index != result[j].index {
			return result[i].index < result[j].index
		}
		return result[i].IPv4 < result[j].IPv4
	})
	return result, false
}

func (s *HTTPServer) webDAVConnection(w http.ResponseWriter, r *http.Request) {
	list, err := interfaceIPv4()
	if err != nil {
		writeErr(w, 500, "cannot enumerate server network addresses")
		return
	}
	local := ""
	if a, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
		local = a.String()
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	addresses, loopback := connectionIPv4(list, s.listen, local, routedIPv4(), scheme)
	preferred := confirmedConnectionURL(addresses, local)
	writeJSON(w, 200, map[string]any{"addresses": addresses, "preferred_url": preferred, "loopback_only": loopback, "selection_required": len(addresses) > 1 && preferred == ""})
}

// A default route ranks the list only. It cannot prove which interface the
// phone can reach when the administrator connects through localhost.
func confirmedConnectionURL(addresses []davIPv4, local string) string {
	host, _, _ := net.SplitHostPort(local)
	for _, address := range addresses {
		if address.IPv4 == host {
			return address.URL
		}
	}
	if len(addresses) == 1 {
		return addresses[0].URL
	}
	return ""
}
