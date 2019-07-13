package main

import (
	"hash/fnv"
	"net"
	"os"

	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type wgState struct {
	iface       string
	client      *wgctrl.Client
	OverlayAddr net.IPNet
	Port        int
	PrivKey     wgtypes.Key
	PubKey      wgtypes.Key
}

func newWGConfig(iface string, port int) (*wgState, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, errors.Wrap(err, "could not instantiate wireguard client")
	}

	privKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	pubKey := privKey.PublicKey()

	wgState := wgState{
		iface:   iface,
		client:  client,
		Port:    port,
		PrivKey: privKey,
		PubKey:  pubKey,
	}
	return &wgState, nil
}

func (wg *wgState) assignOverlayAddr(ipnet *net.IPNet, name string) {
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

func (wg *wgState) downInterface() error {
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

func (wg *wgState) setUpInterface(nodes []node) error {
	if err := wg.createWgInterface(); err != nil {
		return err
	}

	peerCfgs, err := wg.nodesToPeerConfigs(nodes)
	if err != nil {
		return errors.Wrap(err, "error converting received node information to wireguard format")
	}
	wg.client.ConfigureDevice(wg.iface, wgtypes.Config{
		PrivateKey:   &wg.PrivKey,
		ListenPort:   &wg.Port,
		ReplacePeers: true,
		Peers:        peerCfgs,
	})

	link, err := netlink.LinkByName(wg.iface)
	if err != nil {
		return errors.Wrapf(err, "could not get link information for %s", wg.iface)
	}
	netlink.AddrReplace(link, &netlink.Addr{
		IPNet: &wg.OverlayAddr,
	})
	netlink.LinkSetMTU(link, 1420) // TODO: make MTU configurable?
	netlink.LinkSetUp(link)
	for _, node := range nodes {
		netlink.RouteAdd(&netlink.Route{
			LinkIndex: link.Attrs().Index,
			Dst:       &node.OverlayAddr,
			Scope:     netlink.SCOPE_LINK,
		})
	}

	return nil
}

func (wg *wgState) nodesToPeerConfigs(nodes []node) ([]wgtypes.PeerConfig, error) {
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

func (wg *wgState) createWgInterface() error {
	if _, err := wg.client.Device(wg.iface); err == nil {
		// device already exists, but we are running e2e tests, so we're using the user-mode implementation
		// see tests/entrypoint.sh
		if _, e2e := os.LookupEnv("WESHER_E2E_TESTS"); e2e {
			return nil
		}
	}
	if err := netlink.LinkAdd(&wireguard{LinkAttrs: netlink.LinkAttrs{Name: wg.iface}}); err != nil {
		return errors.Wrapf(err, "could not create interface %s", wg.iface)
	}
	return nil
}
