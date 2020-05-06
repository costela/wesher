package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
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

	state := &ClusterState{}
	if !config.Init {
		loadState(state)
	}

	clusterKey, err := computeClusterKey(state, clusterKey)
	if err != nil {
		return nil, err
	}

	bindAddr, err := computeBindAddr(config.BindAddr, config.BindIface)
	if err != nil {
		return nil, err
	}

	mlConfig := memberlist.DefaultWANConfig()
	mlConfig.LogOutput = logrus.StandardLogger().WriterLevel(logrus.DebugLevel)
	mlConfig.SecretKey = clusterKey
	mlConfig.BindAddr = bindAddr
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
		// The big channel buffer is a work-around for https://github.com/hashicorp/memberlist/issues/23
		// More than this many simultaneous events will deadlock cluster.members()
		events: make(chan memberlist.NodeEvent, 100),
		state:  state,
	}
	mlConfig.Conflict = &cluster
	mlConfig.Events = &memberlist.ChannelEventDelegate{Ch: cluster.events}
	mlConfig.Delegate = &cluster

	wg.assignOverlayAddr((*net.IPNet)(config.OverlayNet), cluster.localName)

	ml.UpdateNode(1 * time.Second) // we currently do not update after creation
	return &cluster, nil
}

func (c *cluster) NotifyConflict(node, other *memberlist.Node) {
	logrus.Errorf("node name conflict detected: %s", other.Name)
}

// none of these are used
func (c *cluster) NotifyMsg([]byte)                           {}
func (c *cluster) GetBroadcasts(overhead, limit int) [][]byte { return nil }
func (c *cluster) LocalState(join bool) []byte                { return nil }
func (c *cluster) MergeRemoteState(buf []byte, join bool)     {}

type nodeMeta struct {
	OverlayAddr net.IPNet
	PubKey      string
}

func (c *cluster) NodeMeta(limit int) []byte {
	buf := &bytes.Buffer{}
	if err := gob.NewEncoder(buf).Encode(nodeMeta{
		OverlayAddr: c.wg.OverlayAddr,
		PubKey:      c.wg.PubKey.String(),
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
	// TODO: we blindly trust the info we get from the peers; We should be more defensive to limit the damage a leaked
	// PSK can cause.
	nm := nodeMeta{}
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&nm); err != nil {
		return nm, errors.Wrap(err, "could not decode node meta")
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
	c.ml.Shutdown() //nolint: errcheck
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
					errs = multierr.Append(errs, err)
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

func computeClusterKey(state *ClusterState, clusterKey []byte) ([]byte, error) {
	if len(clusterKey) == 0 {
		clusterKey = state.ClusterKey
	}
	if len(clusterKey) == 0 {
		clusterKey = make([]byte, clusterKeyLen)
		_, err := rand.Read(clusterKey)
		if err != nil {
			return nil, err
		}
		if isatty.IsTerminal(os.Stdout.Fd()) {
			fmt.Printf("new cluster key generated: %s\n", base64.StdEncoding.EncodeToString(clusterKey))
		}
	}
	state.ClusterKey = clusterKey
	return clusterKey, nil
}

func computeBindAddr(bindAddr string, bindIface string) (string, error) {
	if bindIface != "" {
		iface, err := net.InterfaceByName(bindIface)
		if err != nil {
			return "", err
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		if len(addrs) > 0 {
			if addr, ok := addrs[0].(*net.IPNet); ok {
				bindAddr = addr.IP.String()
			}
		}
	}
	return bindAddr, nil
}

func (c *cluster) saveState() error {
	if err := os.MkdirAll(path.Dir(statePath), 0700); err != nil {
		return err
	}

	stateOut, err := json.MarshalIndent(c.state, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(statePath, stateOut, 0600)
}

func loadState(cs *ClusterState) {
	content, err := ioutil.ReadFile(statePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logrus.Warnf("could not open state in %s: %s", statePath, err)
		}
		return
	}

	// avoid partially unmarshalled content by using a temp var
	csTmp := &ClusterState{}
	if err := json.Unmarshal(content, csTmp); err != nil {
		logrus.Warnf("could not decode state: %s", err)
	} else {
		*cs = *csTmp
	}
}
