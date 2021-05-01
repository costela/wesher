package wg

import (
	"hash/fnv"
	"net"
	"os"

	"github.com/costela/wesher/common"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// State holds the configured state of a Wesher Wireguard interface
type State struct {
	iface       string
	client      *wgctrl.Client
	OverlayAddr net.IPNet
	Port        int
	PrivKey     wgtypes.Key
	PubKey      wgtypes.Key
	MTU         int
}

// New creates a new Wesher Wireguard state
// The Wireguard keys are generated for every new interface
// The interface must later be setup using SetUpInterface
func New(iface string, port int, mtu int, ipnet *net.IPNet, name string) (*State, *common.Node, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not instantiate wireguard client")
	}

	privKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, nil, err
	}
	pubKey := privKey.PublicKey()

	state := State{
		iface:   iface,
		client:  client,
		Port:    port,
		PrivKey: privKey,
		PubKey:  pubKey,
		MTU:     mtu,
	}
	state.assignOverlayAddr(ipnet, name)

	node := &common.Node{}
	node.OverlayAddr = state.OverlayAddr
	node.PubKey = state.PubKey.String()

	return &state, node, nil
}

// assignOverlayAddr assigns a new address to the interface
// The address is assigned inside the provided network and depends on the
// provided name deterministically
// Currently, the address is assigned by hashing the name and mapping that
// hash in the target network space
func (s *State) assignOverlayAddr(ipnet *net.IPNet, name string) {
	// TODO: this is way too brittle and opaque
	bits, size := ipnet.Mask.Size()
	ip := make([]byte, len(ipnet.IP))
	copy(ip, []byte(ipnet.IP))

	h := fnv.New128a()
	h.Write([]byte(name))
	hb := h.Sum(nil)

	for i := 1; i <= (size-bits)/8; i++ {
		ip[len(ip)-i] = hb[len(hb)-i]
	}

	s.OverlayAddr = net.IPNet{
		IP:   net.IP(ip),
		Mask: net.CIDRMask(size, size), // either /32 or /128, depending if ipv4 or ipv6
	}
}

// DownInterface shuts down the associated network interface
func (s *State) DownInterface() error {
	if _, err := s.client.Device(s.iface); err != nil {
		if os.IsNotExist(err) {
			return nil // device already gone; noop
		}
		return err
	}
	link, err := netlink.LinkByName(s.iface)
	if err != nil {
		return err
	}
	return netlink.LinkDel(link)
}

// SetUpInterface creates and sets up the associated network interface
func (s *State) SetUpInterface(nodes []common.Node, routedNet []*net.IPNet) error {
	if err := netlink.LinkAdd(&wireguard{LinkAttrs: netlink.LinkAttrs{Name: s.iface}}); err != nil && !os.IsExist(err) {
		return errors.Wrapf(err, "could not create interface %s", s.iface)
	}

	peerCfgs, err := s.nodesToPeerConfigs(nodes)
	if err != nil {
		return errors.Wrap(err, "error converting received node information to wireguard format")
	}
	if err := s.client.ConfigureDevice(s.iface, wgtypes.Config{
		PrivateKey:   &s.PrivKey,
		ListenPort:   &s.Port,
		ReplacePeers: true,
		Peers:        peerCfgs,
	}); err != nil {
		return errors.Wrapf(err, "could not set wireguard configuration for %s", s.iface)
	}

	link, err := netlink.LinkByName(s.iface)
	if err != nil {
		return errors.Wrapf(err, "could not get link information for %s", s.iface)
	}
	if err := netlink.AddrReplace(link, &netlink.Addr{
		IPNet: &s.OverlayAddr,
	}); err != nil {
		return errors.Wrapf(err, "could not set address for %s", s.iface)
	}
	if err := netlink.LinkSetMTU(link, s.MTU); err != nil {
		return errors.Wrapf(err, "could not set MTU for %s", s.iface)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return errors.Wrapf(err, "could not enable interface %s", s.iface)
	}

	// first compute routes
	currentRoutes, err := netlink.RouteList(link, netlink.FAMILY_ALL)
	if err != nil {
		return errors.Wrapf(err, "could not update the routing table for %s", s.iface)
	}
	routes := make([]netlink.Route, 0)
	for index, node := range nodes {
		// dev route
		routes = append(routes, netlink.Route{
			LinkIndex: link.Attrs().Index,
			Dst:       &nodes[index].OverlayAddr,
			Scope:     netlink.SCOPE_LINK,
		})
		// via routes
		for _, route := range node.Routes {
			for _, routedNetItem := range routedNet {
				if !routedNetItem.Contains(route.IP) {
					continue
				}
			}
			routes = append(routes, netlink.Route{
				LinkIndex: link.Attrs().Index,
				Dst:       &route,
				Gw:        node.OverlayAddr.IP,
				Scope:     netlink.SCOPE_SITE,
			})
		}
	}
	// then actually update the routing table
	for _, route := range routes {
		match := matchRoute(currentRoutes, route)
		if match == nil {
			netlink.RouteAdd(&route)
		} else if match.Gw.String() != route.Gw.String() {
			netlink.RouteReplace(&route)
		}
	}
	for _, route := range routes {
		// only delete a reoute if it is a site scope route that belongs to the routed net, mainly to
		// avoid deleting otherwise manually set routes
		for _, routedNetItem := range routedNet {
			if matchRoute(currentRoutes, route) == nil && route.Scope == netlink.SCOPE_LINK && routedNetItem.Contains(route.Dst.IP) {
				netlink.RouteDel(&route)
			}
		}
	}

	return nil
}

func (s *State) nodesToPeerConfigs(nodes []common.Node) ([]wgtypes.PeerConfig, error) {
	peerCfgs := make([]wgtypes.PeerConfig, len(nodes))
	for i, node := range nodes {
		pubKey, err := wgtypes.ParseKey(node.PubKey)
		if err != nil {
			return nil, err
		}
		peerCfgs[i] = wgtypes.PeerConfig{
			PublicKey:         pubKey,
			ReplaceAllowedIPs: true,
			Endpoint: &net.UDPAddr{
				IP:   node.Addr,
				Port: s.Port,
			},
			AllowedIPs: append([]net.IPNet{node.OverlayAddr}, node.Routes...),
		}
	}
	return peerCfgs, nil
}

func matchRoute(set []netlink.Route, needle netlink.Route) *netlink.Route {
	// routes are considered equal if they overlap and have the same prefix length
	prefixn, _ := needle.Dst.Mask.Size()
	for _, route := range set {
		prefixr, _ := route.Dst.Mask.Size()
		if prefixn == prefixr && route.Dst.Contains(needle.Dst.IP) {
			return &route
		}
	}
	return nil
}
