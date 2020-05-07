package common

import (
	"net"
	"reflect"
	"testing"
)

func Test_Node_Encode_Decode(t *testing.T) {
	pubKey := "abcdefghijklmnopkqstuvwxyzABCDEF"
	_, ipv4, _ := net.ParseCIDR("10.0.0.1/32")
	_, ipv6, _ := net.ParseCIDR("2001:db8::1/128")

	for _, ip := range []*net.IPNet{ipv4, ipv6} {
		node := Node{
			nodeMeta: nodeMeta{
				OverlayAddr: *ip,
				PubKey:      pubKey,
			},
		}
		encoded, _ := node.Encode(1024)
		new := Node{Meta: encoded}
		new.Decode()
		if !reflect.DeepEqual(node.nodeMeta, new.nodeMeta) {
			t.Errorf("node encoding then decoding mismatch: %s / %s", node.nodeMeta, new.nodeMeta)
		}
	}
}
