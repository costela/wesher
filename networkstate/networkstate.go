package networkstate

import (
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type State struct {
	sync.RWMutex
	info map[peer.ID]*Info
}

type Info struct {
	LastAnnounce Announce
	Addr         multiaddr.Multiaddr
}

func New() *State {
	return &State{
		info: make(map[peer.ID]*Info),
	}
}

func (s *State) OnAnnounce(from peer.ID, a Announce) {
	s.Lock()
	defer s.Unlock()

	info, ok := s.info[from]
	if !ok {
		info = new(Info)
		s.info[from] = info
	}

	info.LastAnnounce = a
}

func (s *State) UpdateAddrs(addrs map[peer.ID]multiaddr.Multiaddr) {
	s.Lock()
	defer s.Unlock()

	for peer, addr := range addrs {
		info, ok := s.info[peer]
		if !ok {
			info = new(Info)
			s.info[peer] = info
		}

		info.Addr = addr
	}
}

func (s *State) Snapshot() {
	s.RLock()
	defer s.RUnlock()
}
