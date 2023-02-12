package p2p

import (
	"context"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/multiformats/go-multiaddr"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

var log = logging.Logger("w2wesher:p2p")

var keepaliveMessage = []byte(`keepalive`)

const w2wesherTopicName = "w2w:announces"

const announceTimeout = time.Second * 16

type worker struct {
	listenAddr       string
	announceInterval time.Duration
	host             host.Host
	topic            *pubsub.Topic
	pubsub           *pubsub.PubSub
	pk               crypto.PrivKey
	psk              []byte
	bootstrap        []peer.AddrInfo
}

func New(listenAddr string, announceInterval time.Duration, pk crypto.PrivKey, psk []byte, bootstrap []peer.AddrInfo) *worker {
	return &worker{
		listenAddr:       listenAddr,
		announceInterval: announceInterval,
		pk:               pk,
		psk:              psk,
		bootstrap:        bootstrap,
	}
}

func (w *worker) ConnectedPeers(ctx context.Context) map[peer.ID]multiaddr.Multiaddr {
	ret := make(map[peer.ID]multiaddr.Multiaddr)
	n := w.host.Network()

	for _, peer := range n.Peerstore().Peers() {
		for _, addr := range n.ConnsToPeer(peer) {
			ret[peer] = addr.RemoteMultiaddr()

			// we can only use one addr in wg
			break
		}
	}

	return ret
}

func (w *worker) Run(ctx context.Context) error {

	log.Debug("starting")

	// make sure it fails on invalid psk
	pnet.ForcePrivateNetwork = true

	h, err := libp2p.New(
		libp2p.Identity(w.pk),
		libp2p.ListenAddrStrings(w.listenAddr),
		libp2p.PrivateNetwork(w.psk),
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
	)
	if err != nil {
		return err
	}
	w.host = h

	// initial connect to known peers
	for _, addr := range w.bootstrap {
		go func(p peer.AddrInfo) {
			log.With("addr", p).Debug("connecting to the peer")
			err := h.Connect(ctx, p)
			if err != nil {
				log.
					With("addr", p).
					With("err", err).
					Error("failed to connect to the peer")
			}
		}(addr)
	}

	// initialize gossipsub
	ps, err := pubsub.NewGossipSub(ctx, h,
		// this is a small trusted network: enable automatic peer exchange
		pubsub.WithPeerExchange(true),
	)
	if err != nil {
		return err
	}
	w.pubsub = ps

	// join announcements
	topic, err := ps.Join(w2wesherTopicName)
	if err != nil {
		return err
	}
	w.topic = topic

	log.
		With("id", h.ID().String()).
		Info("initialization complete, starting periodic updates")

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)

	wg.Add(1)
	go func() {
		defer cancel()
		defer wg.Done()

		w.periodicAnnounce(ctx)
	}()

	go func() {
		defer cancel()
		defer wg.Done()

		w.consumeAnnounces(ctx)
	}()

	wg.Wait()
	return nil
}

func (w *worker) consumeAnnounces(ctx context.Context) {

	// subscribe to the topic
	sub, err := w.topic.Subscribe()
	if err != nil {
		log.
			With("err", err).
			Error("failed to subscribe to announcements")
		return
	}

	for {
		m, err := sub.Next(ctx)
		if err != nil {
			log.
				With("err", err).
				Error("could not consume a message")
			return
		}

		if m.ReceivedFrom == w.host.ID() {
			continue
		}

		log.
			With("data", string(m.Message.Data)).
			Debug("got announcement")

		var a announce
		err = a.Unmarshal(m.Message.Data)
		if err != nil {
			log.
				With("err", err).
				Error("could not decode the message")
			return
		}

		// connect to the new peer in a non-blocking way
		go func() {
			err := w.host.Connect(ctx, a.AddrInfo)
			if err != nil {
				log.
					With("err", err).
					Error("could not connect to a new peer")
				return
			}
		}()

	}

}

func (w *worker) periodicAnnounce(ctx context.Context) {

	// make a first announce
	w.announceLocal(ctx)

	t := time.NewTicker(w.announceInterval)
	defer t.Stop()

	// periodically announce it's own state
	for {
		select {
		case <-t.C:
			w.announceLocal(ctx)
		case <-ctx.Done():
			return
		}
	}

}

func (w *worker) announceLocal(ctx context.Context) {

	log.Debug("announceLocal")

	ctx, cancel := context.WithTimeout(ctx, announceTimeout)
	defer cancel()

	a := announce{
		AddrInfo: peer.AddrInfo{
			ID:    w.host.ID(),
			Addrs: w.host.Addrs(),
		},
	}

	log.With("announce", a).Debug("going to send announce")

	data, err := a.Marshal()
	if err != nil {
		log.
			With("err", err).
			Error("could not publish keepalive")
		return
	}

	err = w.topic.Publish(ctx, data)
	if err != nil {
		log.
			With("err", err).
			Error("could not publish keepalive")
	}
}
