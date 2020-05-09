package main

import (
	"fmt"
	"net"

	"github.com/hashicorp/go-sockaddr"
	"github.com/stevenroose/gonfig"
)

const clusterKeyLen = 32

type config struct {
	ClusterKey    []byte   `id:"cluster-key" desc:"shared key for cluster membership; must be 32 bytes base64 encoded; will be generated if not provided"`
	Join          []string `desc:"comma separated list of hostnames or IP addresses to existing cluster members; if not provided, will attempt resuming any known state or otherwise wait for further members."`
	Init          bool     `desc:"whether to explicitly (re)initialize the cluster; any known state from previous runs will be forgotten"`
	BindAddr      string   `id:"bind-addr" desc:"IP address to bind to for cluster membership traffic (cannot be used with --bind-iface)"`
	BindIface     string   `id:"bind-iface" desc:"Interface to bind to for cluster membership traffic (cannot be used with --bind-addr)"`
	ClusterPort   int      `id:"cluster-port" desc:"port used for membership gossip traffic (both TCP and UDP); must be the same across cluster" default:"7946"`
	WireguardPort int      `id:"wireguard-port" desc:"port used for wireguard traffic (UDP); must be the same across cluster" default:"51820"`
	OverlayNet    *network `id:"overlay-net" desc:"the network in which to allocate addresses for the overlay mesh network (CIDR format); smaller networks increase the chance of IP collision" default:"10.0.0.0/8"`
	Interface     string   `desc:"name of the wireguard interface to create and manage" default:"wgoverlay"`
	NoEtcHosts    bool     `id:"no-etc-hosts" desc:"disable writing of entries to /etc/hosts"`
	LogLevel      string   `id:"log-level" desc:"set the verbosity (debug/info/warn/error)" default:"warn"`
	Version       bool     `desc:"display current version and exit"`

	// for easier local testing; will break etchosts entry
	UseIPAsName bool `id:"ip-as-name" default:"false" opts:"hidden"`
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

	if config.BindAddr != "" && config.BindIface != "" {
		return nil, fmt.Errorf("setting both bind address and bind interface is not supported")

	} else if config.BindIface != "" {
		// Compute the actual bind address based on the provided interface
		iface, err := net.InterfaceByName(config.BindIface)
		if err != nil {
			return nil, fmt.Errorf("could not get interface by name %s: %w", config.BindIface, err)
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, fmt.Errorf("no available ip address on interface %s", config.BindIface)
		}
		if len(addrs) > 0 {
			if addr, ok := addrs[0].(*net.IPNet); ok {
				config.BindAddr = addr.IP.String()
			}
		}
	} else if config.BindAddr == "" && config.BindIface == "" {
		// FIXME: this is a workaround for memberlist refusing to listen on public IPs if BindAddr==0.0.0.0
		detectedBindAddr, err := sockaddr.GetPublicIP()
		if err != nil {
			return nil, err
		}
		// if we cannot find a public IP, let memberlist do its thing
		if detectedBindAddr != "" {
			config.BindAddr = detectedBindAddr
		} else {
			config.BindAddr = "0.0.0.0"
		}
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
