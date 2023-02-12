package networkstate

import (
	"encoding/json"

	"github.com/libp2p/go-libp2p/core/peer"
)

type Announce struct {
	WireguardPublicKey string
	AddrInfo           peer.AddrInfo `json:"addr_info"`
}

func (a *Announce) Marshal() ([]byte, error) {
	// TODO fastjson
	return json.Marshal(a)
}

func (a *Announce) Unmarshal(data []byte) error {
	// TODO fastjson
	return json.Unmarshal(data, a)
}
