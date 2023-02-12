package networkstate

import (
	"encoding/json"

	"github.com/libp2p/go-libp2p/core/peer"
)

type Announce struct {
	WireguardState WireguardState `json:"wg"`
	AddrInfo       peer.AddrInfo  `json:"ai"`
}

type WireguardState struct {
	WireguardPublicKey string `json:"pk"`
	SelectedAddr       string `json:"ip"`
}

func (a *Announce) Marshal() ([]byte, error) {
	// TODO fastjson
	return json.Marshal(a)
}

func (a *Announce) Unmarshal(data []byte) error {
	// TODO fastjson
	return json.Unmarshal(data, a)
}
