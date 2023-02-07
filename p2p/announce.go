package p2p

import (
	"encoding/json"

	"github.com/libp2p/go-libp2p/core/peer"
)

type announce struct {
	AddrInfo peer.AddrInfo `json:"addr_info"`
}

func (a *announce) Marshal() ([]byte, error) {
	// TODO fastjson
	return json.Marshal(a)
}

func (a *announce) Unmarshal(data []byte) error {
	// TODO fastjson
	return json.Unmarshal(data, a)
}
