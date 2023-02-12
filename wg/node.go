package wg

import (
	"net"
	"net/netip"

	"github.com/libp2p/go-libp2p/core/peer"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type Node struct {
	Endpoint    net.IP
	OverlayAddr netip.Addr
	ID          peer.ID
}

func (n *Node) getWgPubKey() (*wgtypes.Key, error) {

	pubKey, err := n.ID.ExtractPublicKey()
	if err != nil {
		return nil, err
	}

	raw, err := pubKey.Raw()
	if err != nil {
		return nil, err
	}

	parsed, err := wgtypes.NewKey(raw)
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}
