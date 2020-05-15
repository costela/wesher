package main // import "github.com/costela/wesher"

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/costela/wesher/cluster"
	"github.com/costela/wesher/common"
	"github.com/costela/wesher/etchosts"
	"github.com/costela/wesher/wg"
	"github.com/sirupsen/logrus"
)

var version = "dev"

func main() {
	// General initialization
	config, err := loadConfig()
	if err != nil {
		logrus.Fatal(err)
	}
	if config.Version {
		fmt.Println(version)
		os.Exit(0)
	}
	logLevel, err := logrus.ParseLevel(config.LogLevel)
	if err != nil {
		logrus.WithError(err).Fatal("could not parse loglevel")
	}
	logrus.SetLevel(logLevel)

	// Create the wireguard and cluster configuration
	cluster, err := cluster.New(config.Init, config.ClusterKey, config.BindAddr, config.ClusterPort, config.StatePath, config.UseIPAsName)
	if err != nil {
		logrus.WithError(err).Fatal("could not create cluster")
	}
	wgstate, localNode, err := wg.New(config.Interface, config.WireguardPort, (*net.IPNet)(config.OverlayNet), cluster.LocalName)
	if err != nil {
		logrus.WithError(err).Fatal("could not instantiate wireguard controller")
	}

	// Prepare the /etc/hosts writer
	hostsFile := &etchosts.EtcHosts{
		Banner: "# ! managed automatically by wesher " + config.Banner,
		Logger: logrus.StandardLogger(),
	}

	// Join the cluster
	cluster.Update(localNode)
	nodec := cluster.Members() // avoid deadlocks by starting before join
	if err := backoff.RetryNotify(
		func() error { return cluster.Join(config.Join) },
		backoff.NewExponentialBackOff(),
		func(err error, dur time.Duration) {
			logrus.WithError(err).Errorf("could not join cluster, retrying in %s", dur)
		},
	); err != nil {
		logrus.WithError(err).Fatal("could not join cluster")
	}

	// Main loop
	incomingSigs := make(chan os.Signal, 1)
	signal.Notify(incomingSigs, syscall.SIGTERM, os.Interrupt)
	logrus.Debug("waiting for cluster events")
	for {
		select {
		case rawNodes := <-nodec:
			nodes := make([]common.Node, 0, len(rawNodes))
			hosts := make(map[string][]string, len(rawNodes))
			logrus.Info("cluster members:\n")
			for _, node := range rawNodes {
				if err := node.DecodeMeta(); err != nil {
					logrus.Warnf("\t addr: %s, could not decode metadata", node.Addr)
					continue
				}
				logrus.Infof("\taddr: %s, overlay: %s, pubkey: %s", node.Addr, node.OverlayAddr, node.PubKey)
				nodes = append(nodes, node)
				hosts[node.OverlayAddr.IP.String()] = []string{node.Name}
			}
			if err := wgstate.SetUpInterface(nodes); err != nil {
				logrus.WithError(err).Error("could not up interface")
				wgstate.DownInterface()
			}
			if !config.NoEtcHosts {
				if err := hostsFile.WriteEntries(hosts); err != nil {
					logrus.WithError(err).Error("could not write hosts entries")
				}
			}
		case <-incomingSigs:
			logrus.Info("terminating...")
			cluster.Leave()
			if !config.NoEtcHosts {
				if err := hostsFile.WriteEntries(map[string][]string{}); err != nil {
					logrus.WithError(err).Error("could not remove stale hosts entries")
				}
			}
			if err := wgstate.DownInterface(); err != nil {
				logrus.WithError(err).Error("could not down interface")
			}
			os.Exit(0)
		}
	}
}
