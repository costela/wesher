package main

import (
	"fmt"
	"net"

	"github.com/stevenroose/gonfig"
)

const clusterKeyLen = 32

type config struct {
	ClusterKey    []byte   `id:"cluster-key" desc:"shared key for cluster membership; must be 32 bytes base64 encoded; will be generated if not provided"`
	Join          []string `desc:"comma separated list of hostnames or IP addresses to existing cluster members; if not provided, will attempt resuming any known state or otherwise wait for further members."`
	BindAddr      string   `id:"bind-addr" desc:"IP address to bind to for cluster membership" default:"0.0.0.0"`
	ClusterPort   int      `id:"cluster-port" desc:"port used for membership gossip traffic (both TCP and UDP); must be the same across cluster" default:"7946"`
	WireguardPort int      `id:"wireguard-port" desc:"port used for wireguard traffic (UDP); must be the same across cluster" default:"51820"`
	OverlayNet    *network `id:"overlay-net" desc:"the network in which to allocate addresses for the overlay mesh network (CIDR format); smaller networks increase the chance of IP collision" default:"10.0.0.0/8"`
	Interface     string   `desc:"name of the wireguard interface to create and manage" default:"wgoverlay"`
	NoEtcHosts    bool     `id:"no-etc-hosts" desc:"disable writing of entries to /etc/hosts"`
	LogLevel      string   `id:"log-level" desc:"set the verbosity (debug/info/warn/error)" default:"warn"`

	// for easier local testing
	UseIPAsName bool `default:"false" opts:"hidden"`
}

func loadConfig() (*config, error) {
	var config config
	err := gonfig.Load(&config, gonfig.Conf{EnvPrefix: "WESHER_"})
	if err != nil {
		return nil, err
	}

	// perform some validation
	if len(config.ClusterKey) != 0 && len(config.ClusterKey) != clusterKeyLen {
		return nil, fmt.Errorf("unsupported cluster key length; expected %d, got %d", clusterKeyLen, len(config.ClusterKey))
	}

	if bits, _ := ((*net.IPNet)(config.OverlayNet)).Mask.Size(); bits%8 != 0 {
		return nil, fmt.Errorf("unsupported overlay network size; net mask must be multiple of 8, got %d", bits)
	}

	return &config, nil
}

type network net.IPNet

// UnmarshalText parses the provided byte array into the network receiver
func (n *network) UnmarshalText(data []byte) error {
	_, ipnet, err := net.ParseCIDR(string(data))
	if err != nil {
		return err
	}
	*n = network(*ipnet)
	return nil
}
