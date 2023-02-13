package wg

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/derlaft/w2wesher/config"
	"github.com/derlaft/w2wesher/networkstate"
	logging "github.com/ipfs/go-log/v2"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const peerUpdateInterval = time.Minute

var log = logging.Logger("w2wesher:wg")

type Adapter interface {
	Run(context.Context) error
	AnnounceInfo() networkstate.WireguardState
}

func (s *State) Run(ctx context.Context) error {

	err := s.InterfaceUp()
	if err != nil {
		return err
	}

	defer func() {
		err = s.InterfaceDown()
		if err != nil {
			log.With("err", err).Fatal("could not cleanup interface")
		}
	}()

	t := time.NewTicker(peerUpdateInterval)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			// periodic peer update
			err := s.InterfaceUp()
			if err != nil {
				return err
			}
		}
	}
}

// State holds the configured state of a Wesher Wireguard interface.
type State struct {
	iface       string
	client      *wgctrl.Client
	OverlayAddr netip.Addr
	Port        int
	PrivKey     wgtypes.Key
	PubKey      wgtypes.Key
	state       *networkstate.State
}

// New creates a new Wesher Wireguard state.
// The Wireguard keys are generated for every new interface.
// The interface must later be setup using SetUpInterface.
func New(cfg *config.Config, state *networkstate.State) (Adapter, error) {

	c := cfg.Wireguard

	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("instantiating wireguard client: %w", err)
	}

	privKey, err := wgtypes.ParseKey(c.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("loading private key: %w", err)
	}
	pubKey := privKey.PublicKey()

	s := State{
		iface:   c.Interface,
		client:  client,
		Port:    c.ListenPort,
		PrivKey: privKey,
		PubKey:  pubKey,
		state:   state,
	}

	name := c.NodeName
	if name == "" {
		name, _ = os.Hostname()
	}

	prefix, err := netip.ParsePrefix(c.NetworkRange)
	if err != nil {
		return nil, fmt.Errorf("parsing CIDR: %w", err)
	}

	if err := s.assignOverlayAddr(prefix, name); err != nil {
		return nil, fmt.Errorf("assigning overlay address: %w", err)
	}

	return &s, nil
}

// assignOverlayAddr assigns a new address to the interface.
// The address is assigned inside the provided network and depends on the
// provided name deterministically.
// Currently, the address is assigned by hashing the name and mapping that
// hash in the target network space.
func (s *State) assignOverlayAddr(prefix netip.Prefix, name string) error {
	ip := prefix.Addr().AsSlice()

	h := fnv.New128a()
	h.Write([]byte(name))
	hb := h.Sum(nil)

	for i := 1; i <= (prefix.Addr().BitLen()-prefix.Bits())/8; i++ {
		ip[len(ip)-i] = hb[len(hb)-i]
	}

	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return fmt.Errorf("could not create IP from %q", ip)
	}

	log.With("addr", addr).Debug("assigned overlay address")

	s.OverlayAddr = addr

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

// InterfaceUp creates and sets up the associated network interface.
func (s *State) InterfaceUp() error {

	if err := netlink.LinkAdd(&netlink.Wireguard{LinkAttrs: netlink.LinkAttrs{Name: s.iface}}); err != nil && !os.IsExist(err) {
		return fmt.Errorf("creating link %s: %w", s.iface, err)
	}

	nodes := s.state.Snapshot()

	peerCfgs, err := s.peerConfigs(nodes)
	if err != nil {
		return fmt.Errorf("converting received node information to wireguard format: %w", err)
	}

	if err := s.client.ConfigureDevice(s.iface, wgtypes.Config{
		PrivateKey:   &s.PrivKey,
		ListenPort:   &s.Port,
		ReplacePeers: true,
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

func addrToIPNet(addr netip.Addr) *net.IPNet {
	return &net.IPNet{
		IP:   addr.AsSlice(),
		Mask: net.CIDRMask(addr.BitLen(), addr.BitLen()),
	}
}

func (s *State) peerConfigs(nodes []networkstate.Info) ([]wgtypes.PeerConfig, error) {
	peerCfgs := make([]wgtypes.PeerConfig, 0, len(nodes))

	for _, node := range nodes {

		pubKey, err := wgtypes.ParseKey(node.LastAnnounce.WireguardState.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("parsing wireguard key: %w", err)
		}

		selectedAddr, err := netip.ParseAddr(node.LastAnnounce.WireguardState.SelectedAddr)
		if err != nil {
			return nil, fmt.Errorf("parsing selected addr: %w", err)
		}

		peerCfgs = append(peerCfgs, wgtypes.PeerConfig{
			PublicKey:         pubKey,
			ReplaceAllowedIPs: true,
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

func (s *State) AnnounceInfo() networkstate.WireguardState {
	return networkstate.WireguardState{
		PublicKey:    s.PubKey.String(),
		SelectedAddr: s.OverlayAddr.String(),
		Port:         s.Port,
	}
}
