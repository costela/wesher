package state

import (
	"encoding/json"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

type State struct {
	sync.RWMutex
	Peers map[peer.ID]PeerInfo
}

type PeerInfo struct {
	peer.AddrInfo
}

func New() *State {
	return &State{
		Peers: make(map[peer.ID]PeerInfo),
	}
}

func (s *State) Marshal() ([]byte, error) {
	s.RLock()
	defer s.RUnlock()

	// TODO fastjson
	return json.Marshal(s)
}

func (s *State) Update(self peer.ID, pi PeerInfo) {
	s.Lock()
	defer s.Unlock()

	// TODO maybe merge
	s.Peers[self] = pi
}

func (s *State) UnmarshalMerge(networkState []byte, self peer.ID) error {
	s.Lock()
	defer s.Unlock()

	var other State
	err := json.Unmarshal(networkState, &other)
	if err != nil {
		return err
	}

	for k, v := range other.Peers {
		// TODO: maybe merge addrs with old
		if k != self {
			s.Peers[k] = v
		}
	}

	return nil
}
