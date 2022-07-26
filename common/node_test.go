package common

import (
	"net/netip"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Node_Encode_Decode(t *testing.T) {
	pubKey := "abcdefghijklmnopkqstuvwxyzABCDEF"
	ipv4 := netip.MustParseAddr("10.0.0.1")
	ipv6 := netip.MustParseAddr("2001:db8::1")

	for _, ip := range []netip.Addr{ipv4, ipv6} {
		node := Node{
			nodeMeta: nodeMeta{
				OverlayAddr: ip,
				PubKey:      pubKey,
			},
		}
		encoded, _ := node.EncodeMeta(1024)
		new := Node{Meta: encoded}

		err := new.DecodeMeta()
		require.NoError(t, err)

		if !reflect.DeepEqual(node.nodeMeta, new.nodeMeta) {
			t.Errorf("node encoding then decoding mismatch: %s / %s", node.nodeMeta, new.nodeMeta)
		}
	}
}
