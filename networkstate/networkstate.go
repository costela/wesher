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
	Addr         string
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

		// try to extract ipv4 or ipv6 addr
		multiaddr.ForEach(addr, func(c multiaddr.Component) bool {
			switch c.Protocol().Name {
			case "ip4", "ip6":
				info.Addr = c.Value()
				return true
			default:
				return false
			}
		})
	}
}

// Snapshot tries to make a copy which is more or less deep
func (s *State) Snapshot() []Info {
	s.RLock()
	defer s.RUnlock()

	var entries = make([]Info, 0, len(s.info))

	for _, v := range s.info {
		// copy by value
		cp := *v

		entries = append(entries, cp)
	}

	return entries
}
