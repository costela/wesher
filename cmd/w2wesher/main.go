package main

import (
	"context"
	"flag"
	"strings"
	"time"

	"github.com/derlaft/w2wesher/p2p"
	"github.com/derlaft/w2wesher/secret"
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
		for _, rawAddr := range strings.Split(*bootstrapAddrs, ",") {
			addr, err := peer.AddrInfoFromString(rawAddr)
			if err != nil {
				log.Fatal(err)
			}
			bootstrap = append(bootstrap, *addr)
		}
	}

	worker := p2p.New(*listenAddr, *announceInterval, pk, psk, bootstrap)

	err = worker.Start(context.Background())
	if err != nil {
		log.Fatal(err)
	}
}
