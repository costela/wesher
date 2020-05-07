package cluster

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/costela/wesher/common"
	"github.com/hashicorp/memberlist"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const KeyLen = 32

type Cluster struct {
	LocalName string // used to avoid LocalNode(); should not change
	ml        *memberlist.Memberlist
	localNode common.Node
	state     *State
	events    chan memberlist.NodeEvent
}

func New(init bool, clusterKey []byte, bindAddr string, bindPort int, useIPAsName bool) (*Cluster, error) {
	state := &State{}
	if !init {
		loadState(state)
	}

	clusterKey, err := computeClusterKey(state, clusterKey)
	if err != nil {
		return nil, err
	}

	mlConfig := memberlist.DefaultWANConfig()
	mlConfig.LogOutput = logrus.StandardLogger().WriterLevel(logrus.DebugLevel)
	mlConfig.SecretKey = clusterKey
	mlConfig.BindAddr = bindAddr
	mlConfig.BindPort = bindPort
	mlConfig.AdvertisePort = bindPort
	if useIPAsName && bindAddr != "0.0.0.0" {
		mlConfig.Name = bindAddr
	}

	ml, err := memberlist.Create(mlConfig)
	if err != nil {
		return nil, err
	}

	cluster := Cluster{
		LocalName: ml.LocalNode().Name,
		ml:        ml,
		// The big channel buffer is a work-around for https://github.com/hashicorp/memberlist/issues/23
		// More than this many simultaneous events will deadlock cluster.members()
		events: make(chan memberlist.NodeEvent, 100),
		state:  state,
	}
	mlConfig.Conflict = &cluster
	mlConfig.Events = &memberlist.ChannelEventDelegate{Ch: cluster.events}
	mlConfig.Delegate = &cluster

	return &cluster, nil
}

func (c *Cluster) Join(addrs []string) error {
	if len(addrs) == 0 {
		for _, n := range c.state.Nodes {
			addrs = append(addrs, n.Addr.String())
		}
	}

	if _, err := c.ml.Join(addrs); err != nil {
		return err
	} else if len(addrs) > 0 && c.ml.NumMembers() < 2 {
		return errors.New("could not join to any of the provided addresses")
	}
	return nil
}

func (c *Cluster) Leave() {
	c.saveState()
	c.ml.Leave(10 * time.Second)
	c.ml.Shutdown() //nolint: errcheck
}

func (c *Cluster) Update(localNode common.Node) {
	c.localNode = localNode
	c.ml.UpdateNode(1 * time.Second) // we currently do not update after creation
}

func (c *Cluster) Members() <-chan []common.Node {
	changes := make(chan []common.Node)
	go func() {
		for {
			event := <-c.events
			if event.Node.Name == c.LocalName {
				// ignore events about ourselves
				continue
			}
			switch event.Event {
			case memberlist.NodeJoin:
				logrus.Infof("node %s joined", event.Node)
			case memberlist.NodeUpdate:
				logrus.Infof("node %s updated", event.Node)
			case memberlist.NodeLeave:
				logrus.Infof("node %s left", event.Node)
			}

			nodes := make([]common.Node, 0)
			for _, n := range c.ml.Members() {
				if n.Name == c.LocalName {
					continue
				}
				nodes = append(nodes, common.Node{
					Name: n.Name,
					Addr: n.Addr,
					Meta: n.Meta,
				})
			}
			c.state.Nodes = nodes
			changes <- nodes
			c.saveState()
		}
	}()
	return changes
}

func computeClusterKey(state *State, clusterKey []byte) ([]byte, error) {
	if len(clusterKey) == 0 {
		clusterKey = state.ClusterKey
	}
	if len(clusterKey) == 0 {
		clusterKey = make([]byte, KeyLen)
		_, err := rand.Read(clusterKey)
		if err != nil {
			return nil, err
		}
		// TODO: refactor this into subcommand ("showkey"?)
		if isatty.IsTerminal(os.Stdout.Fd()) {
			fmt.Printf("new cluster key generated: %s\n", base64.StdEncoding.EncodeToString(clusterKey))
		}
	}
	state.ClusterKey = clusterKey
	return clusterKey, nil
}
