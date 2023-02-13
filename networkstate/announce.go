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
	PublicKey    string `json:"pk"`
	SelectedAddr string `json:"ip"`
	Port         int    `json:"port"`
}

func (ws WireguardState) IsValid() bool {
	return ws.PublicKey > "" && ws.SelectedAddr > "" && ws.Port > 0
}

func (a *Announce) Marshal() ([]byte, error) {
	// TODO fastjson
	return json.Marshal(a)
}

func (a *Announce) Unmarshal(data []byte) error {
	// TODO fastjson
	return json.Unmarshal(data, a)
}
