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

// KeyLen is the fixed length of cluster keys, must be checked by callers
const KeyLen = 32

// Cluster represents a running cluster configuration
type Cluster struct {
	ml        *memberlist.Memberlist
	mlConfig  *memberlist.Config
	localNode *common.Node
	LocalName string
	statePath string
	state     *state
	events    chan memberlist.NodeEvent
}

// New is used to create a new Cluster instance
// The returned instance is ready to be updated with the local node settings then joined
func New(init bool, clusterKey []byte, bindAddr string, bindPort int, statePath string, useIPAsName bool) (*Cluster, error) {
	state := &state{}
	if !init {
		loadState(state, statePath)
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
		ml:        ml,
		mlConfig:  mlConfig,
		LocalName: ml.LocalNode().Name,
		// The big channel buffer is a work-around for https://github.com/hashicorp/memberlist/issues/23
		// More than this many simultaneous events will deadlock cluster.members()
		events:    make(chan memberlist.NodeEvent, 100),
		state:     state,
		statePath: statePath,
	}
	return &cluster, nil
}

// Name provides the current cluster name
func (c *Cluster) Name() string {
	return c.localNode.Name
}

// Join tries to join the cluster by contacting provided addresses
// Provided addresses are passed as is, if no address is provided, known
// cluster nodes are contacted instead.
// Joining fail if none of the provided addresses or none of the known
// nodes can be joined.
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

// Leave saves the current state before leaving, then leaves the cluster
func (c *Cluster) Leave() {
	c.state.save(c.statePath)
	c.ml.Leave(10 * time.Second)
	c.ml.Shutdown() //nolint: errcheck
}

// Update gossips the local node configuration, propagating any change
func (c *Cluster) Update(localNode *common.Node) {
	c.localNode = localNode
	// wrap in a delegateNode instance for memberlist.Delegate implementation
	delegate := &delegateNode{c.localNode}
	c.mlConfig.Conflict = delegate
	c.mlConfig.Delegate = delegate
	c.mlConfig.Events = &memberlist.ChannelEventDelegate{Ch: c.events}
	c.ml.UpdateNode(1 * time.Second) // we currently do not update after creation
}

// Members provides a channel notifying of cluster changes
// Everytime a change happens inside the cluster (except for local changes),
// the updated list of cluster nodes is pushed to the channel.
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
			c.state.save(c.statePath)
		}
	}()
	return changes
}

func computeClusterKey(state *state, clusterKey []byte) ([]byte, error) {
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
