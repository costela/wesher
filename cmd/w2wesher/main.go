package main

import (
	"context"
	"flag"
	"time"

	"github.com/derlaft/w2wesher/p2p"
	"github.com/derlaft/w2wesher/secret"
	"github.com/derlaft/w2wesher/state"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("w2wesher")

var (
	listenAddr       = flag.String("listenAddr", "/ip4/0.0.0.0/tcp/10042", "listen address")
	announceInterval = flag.Duration("announceInterval", time.Minute, "interval for periodic state announcements")
	identityFile     = flag.String("identityFile", ".w2wesher.secret", "secret identity file (will be automatically created if not exists)")
	pskFile          = flag.String("pskFile", ".w2wesher.psk", "psk file (will not be automatically created; plaintext)")
)

func main() {
	flag.Parse()

	s := state.New()

	pk, err := secret.LoadOrCreate(*identityFile)
	if err != nil {
		log.Fatal(err)
	}

	psk, err := secret.LoadPsk(*pskFile)
	if err != nil {
		log.Fatal(err)
	}

	worker := p2p.New(*listenAddr, *announceInterval, pk, psk, s)

	err = worker.Start(context.Background())
	if err != nil {
		log.Fatal(err)
	}
}
