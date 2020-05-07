package wg

import "github.com/vishvananda/netlink"

// this is only necessary while this PR is open:
// https://github.com/vishvananda/netlink/pull/464

type wireguard struct {
	netlink.LinkAttrs
}

func (wg *wireguard) Attrs() *netlink.LinkAttrs {
	return &wg.LinkAttrs
}

func (wg *wireguard) Type() string {
	return "wireguard"
}
