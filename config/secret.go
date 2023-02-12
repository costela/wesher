package config

import (
	"encoding/base64"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"gopkg.in/ini.v1"
)

var log = logging.Logger("w2wesher")

// TODO: validate config on load
type Config struct {
	P2P       P2P
	Wireguard Wireguard
}

type P2P struct {
	PSK              string
	PrivateKey       string
	Bootstrap        []string
	ListenAddr       string
	AnnounceInterval time.Duration
}

type Wireguard struct {
	Interface    string
	PrivateKey   string
	ListenPort   int
	NetworkRange string
	NodeName     string
}

func Load(filename string) (*Config, error) {

	var parsed = new(Config)
	err := ini.MapTo(parsed, filename)
	if err != nil {
		return nil, fmt.Errorf("config: cannot map ini: %w", err)
	}

	return parsed, nil
}

func (p *P2P) LoadPrivateKey() (crypto.PrivKey, error) {

	data, err := base64.StdEncoding.DecodeString(p.PrivateKey)
	if err != nil {
		return nil, err
	}

	privateKey, err := crypto.UnmarshalPrivateKey(data)
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

func (p *P2P) LoadPsk() ([]byte, error) {
	return base64.StdEncoding.DecodeString(p.PSK)
}

func (p *P2P) LoadBootstrapPeers() ([]peer.AddrInfo, error) {

	var bootstrap []peer.AddrInfo
	for _, rawAddr := range p.Bootstrap {
		addr, err := peer.AddrInfoFromString(rawAddr)
		if err != nil {
			return nil, fmt.Errorf("config: invalid bootstrap addr %v: %w", rawAddr, err)
		}
		bootstrap = append(bootstrap, *addr)
	}

	log.With("bootstrap", bootstrap).Debug("loaded boostrap peers")

	return bootstrap, nil
}
