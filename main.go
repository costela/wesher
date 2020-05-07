package main // import "github.com/costela/wesher"

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/costela/wesher/etchosts"
	"github.com/sirupsen/logrus"
)

var version = "dev"

func main() {
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

	wg, err := newWGConfig(config.Interface, config.WireguardPort)
	if err != nil {
		logrus.WithError(err).Fatal("could not instantiate wireguard controller")
	}

	getMeta := func(limit int) []byte {
		return encodeNodeMeta(nodeMeta{
			OverlayAddr: wg.OverlayAddr,
			PubKey:      wg.PubKey.String(),
		}, limit)
	}

	cluster, err := newCluster(config.Init, config.ClusterKey, config.BindAddr, config.ClusterPort, config.UseIPAsName, getMeta)
	if err != nil {
		logrus.WithError(err).Fatal("could not create cluster")
	}
	wg.assignOverlayAddr((*net.IPNet)(config.OverlayNet), cluster.localName)
	cluster.update()

	nodec := cluster.members() // avoid deadlocks by starting before join
	if err := backoff.RetryNotify(
		func() error { return cluster.join(config.Join) },
		backoff.NewExponentialBackOff(),
		func(err error, dur time.Duration) {
			logrus.WithError(err).Errorf("could not join cluster, retrying in %s", dur)
		},
	); err != nil {
		logrus.WithError(err).Fatal("could not join cluster")
	}

	incomingSigs := make(chan os.Signal, 1)
	signal.Notify(incomingSigs, syscall.SIGTERM, os.Interrupt)
	logrus.Debug("waiting for cluster events")
	for {
		select {
		case rawNodes := <-nodec:
			logrus.Info("cluster members:\n")
			nodes := make([]node, 0, len(rawNodes))
			for _, node := range rawNodes {
				meta, err := decodeNodeMeta(node.Meta)
				if err != nil {
					logrus.Warnf("\t addr: %s, could not decode metadata", node.Addr)
					continue
				}
				node.nodeMeta = meta
				nodes = append(nodes, node)
				logrus.Infof("\taddr: %s, overlay: %s, pubkey: %s", node.Addr, node.OverlayAddr, node.PubKey)
			}
			if err := wg.setUpInterface(nodes); err != nil {
				logrus.WithError(err).Error("could not up interface")
				wg.downInterface()
			}
			if !config.NoEtcHosts {
				if err := writeToEtcHosts(nodes); err != nil {
					logrus.WithError(err).Error("could not write hosts entries")
				}
			}
		case <-incomingSigs:
			logrus.Info("terminating...")
			cluster.leave()
			if !config.NoEtcHosts {
				if err := writeToEtcHosts(nil); err != nil {
					logrus.WithError(err).Error("could not remove stale hosts entries")
				}
			}
			if err := wg.downInterface(); err != nil {
				logrus.WithError(err).Error("could not down interface")
			}
			os.Exit(0)
		}
	}
}

func writeToEtcHosts(nodes []node) error {
	hosts := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		hosts[n.OverlayAddr.IP.String()] = []string{n.Name}
	}
	hostsFile := &etchosts.EtcHosts{
		Logger: logrus.StandardLogger(),
	}
	return hostsFile.WriteEntries(hosts)
}
