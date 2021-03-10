package common

import (
	"net"

	"github.com/vishvananda/netlink"
)

// Routes pushes list of local routes to a channel, after filtering using the provided network
// The full list is pushed after every routing change
func Routes(filter *net.IPNet) <-chan []net.IPNet {
	routesc := make(chan []net.IPNet)
	updatec := make(chan netlink.RouteUpdate)
	netlink.RouteSubscribe(updatec, make(chan struct{}))
	go func() {
		for {
			<-updatec
			routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
			if err != nil {
				continue
			}
			result := make([]net.IPNet, 0)
			for _, route := range routes {
				if route.Dst != nil && filter.Contains(route.Dst.IP) {
					result = append(result, *route.Dst)
				}
			}
			routesc <- result
		}
	}()
	return routesc
}
