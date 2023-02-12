package main

import (
	"context"
	"flag"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/derlaft/w2wesher/p2p"
	"github.com/derlaft/w2wesher/runnergroup"
	"github.com/derlaft/w2wesher/secret"
	"github.com/derlaft/w2wesher/wg"
	"github.com/libp2p/go-libp2p/core/peer"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("w2wesher")

var (
	listenAddr       = flag.String("listenAddr", "/ip4/0.0.0.0/tcp/10042", "listen address")
	announceInterval = flag.Duration("announceInterval", time.Minute, "interval for periodic state announcements")
	identityFile     = flag.String("identityFile", ".w2wesher.secret", "secret identity file (will be automatically created if not exists)")
	pskFile          = flag.String("pskFile", ".w2wesher.psk", "psk file (will not be automatically created; plaintext)")
	bootstrapAddrs   = flag.String("bootstrapAddrs", "", "comma-separated list of bootstrap addrs")
)

func main() {
	flag.Parse()

	pk, err := secret.LoadOrCreate(*identityFile)
	if err != nil {
		log.Fatal(err)
	}

	psk, err := secret.LoadPsk(*pskFile)
	if err != nil {
		log.Fatal(err)
	}

	var bootstrap []peer.AddrInfo
	if *bootstrapAddrs > "" {
		// TODO: support multiple bootstrap nodes... somehow
		for _, rawAddr := range strings.Split(*bootstrapAddrs, ",") {
			addr, err := peer.AddrInfoFromString(rawAddr)
			if err != nil {
				log.
					With("addr", rawAddr).
					With("err", err).
					Fatal("could not parse bootstrap addr")
			}
			bootstrap = append(bootstrap, *addr)
		}
	}

	log.With("bs", bootstrap).Debug("wat")

	node := p2p.New(*listenAddr, *announceInterval, pk, psk, bootstrap)

	adapter, err := wg.New(
		pk,
		/* iface name: TODO make it configurable */ "w2wesher",
		/* port name: TODO make it configurable */ 10043,
		/* prefix: TODO make it configurable */ netip.MustParsePrefix("10.42.0.0/24"),
		os.Getenv("HOSTNAME"),
	)
	if err != nil {
		log.Fatal(err)
	}

	defer adapter.DownInterface()

	err = runnergroup.New(context.TODO()).
		Go(node.Run).
		Go(adapter.Run).
		Wait()
	if err != nil {
		log.Error(err)
	}
}
