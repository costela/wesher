package p2p

import (
	"context"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/pnet"

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

func (w *worker) Start(ctx context.Context) error {

	log.Debug("starting")

	// make sure it fails on invalid psk
	pnet.ForcePrivateNetwork = true

	h, err := libp2p.New(
		libp2p.Identity(w.pk),
		libp2p.ListenAddrStrings(w.listenAddr),
		libp2p.PrivateNetwork(w.psk),
		// TODO: nat?
	)
	if err != nil {
		return err
	}
	w.host = h

	// initial connect to known peers
	for _, addr := range w.bootstrap {
		go func(addr peer.AddrInfo) {
			log.
				With("addr", addr).
				Debug("connecting to the peer")

			err := h.Connect(ctx, addr)
			if err != nil {
				log.
					With("addr", addr).
					Error("failed to connect to the peer")
			}
		}(addr)
	}

	// initialize gossipsub
	ps, err := pubsub.NewGossipSub(ctx, h)
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

	// subscribe to the topic
	sub, err := topic.Subscribe()
	if err != nil {
		return err
	}

	_ = sub
	log.
		With("id", h.ID()).
		With("addrs", h.Addrs()).
		Info("initialization complete, starting periodic updates")

	w.periodicAnnounce(ctx)

	return nil
}

func (w worker) periodicAnnounce(ctx context.Context) {

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

	err := w.topic.Publish(ctx, keepaliveMessage)
	if err != nil {
		log.
			With("err", err).
			Error("could not publish keepalive")
	}
}
