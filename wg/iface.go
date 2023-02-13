package wg

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"

	"github.com/derlaft/w2wesher/networkstate"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// InterfaceUp creates and sets up the associated network interface.
func (s *State) InterfaceUp() error {

	log.Info("InterfaceUp")

	if err := netlink.LinkAdd(&netlink.Wireguard{LinkAttrs: netlink.LinkAttrs{Name: s.iface}}); err != nil && !os.IsExist(err) {
		return fmt.Errorf("creating link %s: %w", s.iface, err)
	}

	nodes := s.state.Snapshot()

	peerCfgs, err := s.peerConfigs(nodes)
	if err != nil {
		return fmt.Errorf("converting received node information to wireguard format: %w", err)
	}

	if err := s.client.ConfigureDevice(s.iface, wgtypes.Config{
		PrivateKey: &s.PrivKey,
		ListenPort: &s.Port,
		// even if libp2p connection is broken, we want to keep the old peers
		// to have the best connectivity chances
		ReplacePeers: false,
		Peers:        peerCfgs,
	}); err != nil {
		return fmt.Errorf("setting wireguard configuration for %s: %w", s.iface, err)
	}

	link, err := netlink.LinkByName(s.iface)
	if err != nil {
		return fmt.Errorf("getting link information for %s: %w", s.iface, err)
	}

	if err := netlink.AddrReplace(link, &netlink.Addr{
		IPNet: addrToIPNet(s.OverlayAddr),
	}); err != nil {
		return fmt.Errorf("setting address for %s: %w", s.iface, err)
	}

	// TODO: make MTU configurable?
	if err := netlink.LinkSetMTU(link, 1420); err != nil {
		return fmt.Errorf("setting MTU for %s: %w", s.iface, err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("enabling interface %s: %w", s.iface, err)
	}

	for _, node := range nodes {

		if !node.LastAnnounce.WireguardState.IsValid() {
			continue
		}

		selectedAddr, err := netip.ParseAddr(node.LastAnnounce.WireguardState.SelectedAddr)
		if err != nil {
			return fmt.Errorf("parsing selected addr: %w", err)
		}

		if err := netlink.RouteAdd(&netlink.Route{
			LinkIndex: link.Attrs().Index,
			Dst:       addrToIPNet(selectedAddr),
			Scope:     netlink.SCOPE_LINK,
		}); err != nil && !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("adding route %s to %s: %w", selectedAddr, s.iface, err)
		}
	}

	return nil
}

// InterfaceDown shuts down the associated network interface.
func (s *State) InterfaceDown() error {
	if _, err := s.client.Device(s.iface); err != nil {
		if os.IsNotExist(err) {
			return nil // device already gone; noop
		}
		return fmt.Errorf("getting device %s: %w", s.iface, err)
	}
	link, err := netlink.LinkByName(s.iface)
	if err != nil {
		return fmt.Errorf("getting link for %s: %w", s.iface, err)
	}
	return netlink.LinkDel(link)
}

func (s *State) peerConfigs(nodes []networkstate.Info) ([]wgtypes.PeerConfig, error) {
	peerCfgs := make([]wgtypes.PeerConfig, 0, len(nodes))

	for _, node := range nodes {

		a := node.LastAnnounce

		if !a.WireguardState.IsValid() {
			// have not received an announce from that node just yet
			continue
		}

		pubKey, err := wgtypes.ParseKey(a.WireguardState.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("parsing wireguard key: %w", err)
		}

		selectedAddr, err := netip.ParseAddr(node.LastAnnounce.WireguardState.SelectedAddr)
		if err != nil {
			return nil, fmt.Errorf("parsing selected addr: %w", err)
		}

		peerCfgs = append(peerCfgs, wgtypes.PeerConfig{
			PublicKey:                   pubKey,
			ReplaceAllowedIPs:           true,
			PersistentKeepaliveInterval: s.persistentKeepalive,
			Endpoint: &net.UDPAddr{
				IP:   net.ParseIP(node.Addr),
				Port: s.Port,
			},
			AllowedIPs: []net.IPNet{
				*addrToIPNet(selectedAddr),
			},
		})
	}

	return peerCfgs, nil
}
