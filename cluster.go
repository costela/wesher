package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"time"

	"github.com/mattn/go-isatty"

	"github.com/hashicorp/errwrap"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"
)

// ClusterState keeps track of information needed to rejoin the cluster
type ClusterState struct {
	ClusterKey []byte
	Nodes      []node
}

type cluster struct {
	localName string // used to avoid LocalNode(); should not change
	ml        *memberlist.Memberlist
	wg        *wgState
	state     *ClusterState
	events    chan memberlist.NodeEvent
}

const statePath = "/var/lib/wesher/state.json"

func newCluster(config *config, wg *wgState) (*cluster, error) {
	clusterKey := config.ClusterKey

	state := loadState()
	if len(clusterKey) == 0 {
		clusterKey = state.ClusterKey
	}

	if len(clusterKey) == 0 {
		clusterKey = make([]byte, clusterKeyLen)
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

	mlConfig := memberlist.DefaultWANConfig()
	mlConfig.LogOutput = logrus.StandardLogger().WriterLevel(logrus.DebugLevel)
	mlConfig.SecretKey = clusterKey
	mlConfig.BindAddr = config.BindAddr
	mlConfig.BindPort = config.ClusterPort
	mlConfig.AdvertisePort = config.ClusterPort
	if config.UseIPAsName && config.BindAddr != "0.0.0.0" {
		mlConfig.Name = config.BindAddr
	}

	ml, err := memberlist.Create(mlConfig)
	if err != nil {
		return nil, err
	}

	cluster := cluster{
		localName: ml.LocalNode().Name,
		ml:        ml,
		wg:        wg,
		events:    make(chan memberlist.NodeEvent, 1),
		state:     state,
	}
	mlConfig.Conflict = &cluster
	mlConfig.Events = &memberlist.ChannelEventDelegate{Ch: cluster.events}
	mlConfig.Delegate = &cluster

	wg.assignIP((*net.IPNet)(config.OverlayNet), cluster.localName)

	ml.UpdateNode(1 * time.Second) // we currently do not update after creation
	return &cluster, nil
}

func (c *cluster) NotifyConflict(node, other *memberlist.Node) {
	logrus.Errorf("node name conflict detected: %s", other.Name)
}

// none if these are used
func (c *cluster) NotifyMsg([]byte)                           {}
func (c *cluster) GetBroadcasts(overhead, limit int) [][]byte { return nil }
func (c *cluster) LocalState(join bool) []byte                { return nil }
func (c *cluster) MergeRemoteState(buf []byte, join bool)     {}

type nodeMeta struct {
	OverlayAddr net.IP
	PubKey      string
}

func (c *cluster) NodeMeta(limit int) []byte {
	buf := &bytes.Buffer{}
	if err := gob.NewEncoder(buf).Encode(nodeMeta{
		OverlayAddr: c.wg.OverlayAddr,
		PubKey:      c.wg.PubKey,
	}); err != nil {
		logrus.Errorf("could not encode local state: %s", err)
		return nil
	}
	if buf.Len() > limit {
		logrus.Errorf("could not fit node metadata into %d bytes", limit)
		return nil
	}
	return buf.Bytes()
}

func decodeNodeMeta(b []byte) (nodeMeta, error) {
	nm := nodeMeta{}
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&nm); err != nil {
		return nm, errwrap.Wrapf("could not decode: {{err}}", err)
	}
	return nm, nil
}

func (c *cluster) join(addrs []string) error {
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

func (c *cluster) leave() {
	c.saveState()
	c.ml.Leave(10 * time.Second)
	c.ml.Shutdown() // ignore errors
}

func (c *cluster) members() (<-chan []node, <-chan error) {
	changes := make(chan []node)
	errc := make(chan error, 1)
	go func() {
		for {
			event := <-c.events
			if event.Node.Name == c.localName {
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

			nodes := make([]node, 0)
			var errs error
			for _, n := range c.ml.Members() {
				if n.Name == c.localName {
					continue
				}
				meta, err := decodeNodeMeta(n.Meta)
				if err != nil {
					errs = multierror.Append(errs, err)
					continue
				}
				nodes = append(nodes, node{
					Name:     n.Name,
					Addr:     n.Addr,
					nodeMeta: meta,
				})
			}
			c.state.Nodes = nodes
			changes <- nodes
			if errs != nil {
				errc <- errs
			}
			c.saveState()
		}
	}()
	return changes, errc
}

type node struct {
	Name string
	Addr net.IP
	nodeMeta
}

func (n *node) String() string {
	return n.Addr.String()
}

func (c *cluster) saveState() error {
	if err := os.MkdirAll(path.Dir(statePath), 0700); err != nil {
		return err
	}

	stateOut, err := json.MarshalIndent(c.state, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(statePath, stateOut, 0700)
}

func loadState() *ClusterState {
	content, err := ioutil.ReadFile(statePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logrus.Warnf("could not open state in %s: %s", statePath, err)
		}
		return &ClusterState{}
	}

	s := &ClusterState{}
	if err := json.Unmarshal(content, s); err != nil {
		logrus.Warnf("could not decode state: %s", err)
		return &ClusterState{} // avoid partially unmarshalled content
	}
	return s
}
