package main // import "github.com/costela/wesher"

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	"github.com/costela/wesher/etchosts"
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
		logrus.Fatalf("could not parse loglevel: %s", err)
	}
	logrus.SetLevel(logLevel)

	wg, err := newWGConfig(config.Interface, config.WireguardPort)
	if err != nil {
		logrus.Fatal(err)
	}

	cluster, err := newCluster(config, wg)
	if err != nil {
		logrus.Fatalf("could not create cluster: %s", err)
	}

	nodec, errc := cluster.members() // avoid deadlocks by starting before join
	if err := cluster.join(config.Join); err != nil {
		logrus.Fatalf("could not join cluster: %s", err)
	}

	incomingSigs := make(chan os.Signal, 1)
	signal.Notify(incomingSigs, syscall.SIGTERM, os.Interrupt)
	for {
		select {
		case nodes := <-nodec:
			logrus.Info("cluster members:\n")
			for _, node := range nodes {
				logrus.Infof("\taddr: %s, overlay: %s, pubkey: %s", node.Addr, node.OverlayAddr, node.PubKey)
			}
			if err := wg.downInterface(); err != nil {
				logrus.Errorf("could not down interface: %s", err)
			}
			if err := wg.writeConf(nodes); err != nil {
				logrus.Errorf("could not write config: %s", err)
			}
			if err := wg.upInterface(); err != nil {
				logrus.Errorf("could not up interface: %s", err)
			}
			if !config.NoEtcHosts {
				if err := writeToEtcHosts(nodes); err != nil {
					logrus.Errorf("could not write hosts entries: %s", err)
				}
			}
		case errs := <-errc:
			logrus.Errorf("could not receive node info: %s", errs)
		case <-incomingSigs:
			logrus.Info("terminating...")
			cluster.leave()
			if err := writeToEtcHosts(nil); err != nil {
				logrus.Errorf("could not remove stale hosts entries: %s", err)
			}
			if err := wg.downInterface(); err != nil {
				logrus.Errorf("could not down interface: %s", err)
			}
			os.Exit(0)
		}
	}
}

func writeToEtcHosts(nodes []node) error {
	hosts := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		hosts[n.OverlayAddr.String()] = []string{n.Name}
	}
	hostsFile := &etchosts.EtcHosts{
		Logger: logrus.StandardLogger(),
	}
	return hostsFile.WriteEntries(hosts)
}
