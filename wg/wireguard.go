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

// WgState holds the configured state of a Wesher Wireguard interface
type WgState struct {
	iface       string
	client      *wgctrl.Client
	OverlayAddr net.IPNet
	Port        int
	PrivKey     wgtypes.Key
	PubKey      wgtypes.Key
}

// NewWGConfig creates a new Wesher Wireguard state
// The Wireguard keys are generated for every new interface
// The interface must later be setup using SetUpInterface
func NewWGConfig(iface string, port int) (*WgState, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, errors.Wrap(err, "could not instantiate wireguard client")
	}

	privKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	pubKey := privKey.PublicKey()

	wgState := WgState{
		iface:   iface,
		client:  client,
		Port:    port,
		PrivKey: privKey,
		PubKey:  pubKey,
	}
	return &wgState, nil
}

// AssignOverlayAddr assigns a new address to the interface
// The address is assigned inside the provided network and depends on the
// provided name deterministically
// Currently, the address is assigned by hashing the name and mapping that
// hash in the target network space
func (wg *WgState) AssignOverlayAddr(ipnet *net.IPNet, name string) {
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

	wg.OverlayAddr = net.IPNet{
		IP:   net.IP(ip),
		Mask: net.CIDRMask(size, size), // either /32 or /128, depending if ipv4 or ipv6
	}
}

// DownInterface shuts down the associated network interface
func (wg *WgState) DownInterface() error {
	if _, err := wg.client.Device(wg.iface); err != nil {
		if os.IsNotExist(err) {
			return nil // device already gone; noop
		}
		return err
	}
	link, err := netlink.LinkByName(wg.iface)
	if err != nil {
		return err
	}
	return netlink.LinkDel(link)
}

// SetUpInterface creates and setup the associated network interface
func (wg *WgState) SetUpInterface(nodes []common.Node) error {
	if err := netlink.LinkAdd(&wireguard{LinkAttrs: netlink.LinkAttrs{Name: wg.iface}}); err != nil && !os.IsExist(err) {
		return errors.Wrapf(err, "could not create interface %s", wg.iface)
	}

	peerCfgs, err := wg.nodesToPeerConfigs(nodes)
	if err != nil {
		return errors.Wrap(err, "error converting received node information to wireguard format")
	}
	if err := wg.client.ConfigureDevice(wg.iface, wgtypes.Config{
		PrivateKey:   &wg.PrivKey,
		ListenPort:   &wg.Port,
		ReplacePeers: true,
		Peers:        peerCfgs,
	}); err != nil {
		return errors.Wrapf(err, "could not set wireguard configuration for %s", wg.iface)
	}

	link, err := netlink.LinkByName(wg.iface)
	if err != nil {
		return errors.Wrapf(err, "could not get link information for %s", wg.iface)
	}
	if err := netlink.AddrReplace(link, &netlink.Addr{
		IPNet: &wg.OverlayAddr,
	}); err != nil {
		return errors.Wrapf(err, "could not set address for %s", wg.iface)
	}
	// TODO: make MTU configurable?
	if err := netlink.LinkSetMTU(link, 1420); err != nil {
		return errors.Wrapf(err, "could not set MTU for %s", wg.iface)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return errors.Wrapf(err, "could not enable interface %s", wg.iface)
	}
	for _, node := range nodes {
		netlink.RouteAdd(&netlink.Route{
			LinkIndex: link.Attrs().Index,
			Dst:       &node.OverlayAddr,
			Scope:     netlink.SCOPE_LINK,
		})
	}

	return nil
}

func (wg *WgState) nodesToPeerConfigs(nodes []common.Node) ([]wgtypes.PeerConfig, error) {
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
				Port: wg.Port,
			},
			AllowedIPs: []net.IPNet{
				node.OverlayAddr,
			},
		}
	}
	return peerCfgs, nil
}
