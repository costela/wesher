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

	wg, err := wg.NewWGConfig(config.Interface, config.WireguardPort)
	if err != nil {
		logrus.WithError(err).Fatal("could not instantiate wireguard controller")
	}

	cluster, err := cluster.New(config.Init, config.ClusterKey, config.BindAddr, config.ClusterPort, config.UseIPAsName)
	if err != nil {
		logrus.WithError(err).Fatal("could not create cluster")
	}

	wg.AssignOverlayAddr((*net.IPNet)(config.OverlayNet), cluster.LocalName)
	localNode := common.MakeLocalNode(wg.OverlayAddr, wg.PubKey.String())
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

	incomingSigs := make(chan os.Signal, 1)
	signal.Notify(incomingSigs, syscall.SIGTERM, os.Interrupt)
	logrus.Debug("waiting for cluster events")
	for {
		select {
		case rawNodes := <-nodec:
			logrus.Info("cluster members:\n")
			nodes := make([]common.Node, 0, len(rawNodes))
			for _, node := range rawNodes {
				if err := node.Decode(); err != nil {
					logrus.Warnf("\t addr: %s, could not decode metadata", node.Addr)
					continue
				}
				nodes = append(nodes, node)
				logrus.Infof("\taddr: %s, overlay: %s, pubkey: %s", node.Addr, node.OverlayAddr, node.PubKey)
			}
			if err := wg.SetUpInterface(nodes); err != nil {
				logrus.WithError(err).Error("could not up interface")
				wg.DownInterface()
			}
			if !config.NoEtcHosts {
				if err := writeToEtcHosts(nodes); err != nil {
					logrus.WithError(err).Error("could not write hosts entries")
				}
			}
		case <-incomingSigs:
			logrus.Info("terminating...")
			cluster.Leave()
			if !config.NoEtcHosts {
				if err := writeToEtcHosts(nil); err != nil {
					logrus.WithError(err).Error("could not remove stale hosts entries")
				}
			}
			if err := wg.DownInterface(); err != nil {
				logrus.WithError(err).Error("could not down interface")
			}
			os.Exit(0)
		}
	}
}

func writeToEtcHosts(nodes []common.Node) error {
	hosts := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		hosts[n.OverlayAddr.IP.String()] = []string{n.Name}
	}
	hostsFile := &etchosts.EtcHosts{
		Logger: logrus.StandardLogger(),
	}
	return hostsFile.WriteEntries(hosts)
}
