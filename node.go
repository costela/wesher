package main

import (
	"bytes"
	"encoding/gob"
	"net"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// nodeMeta holds metadata sent over the cluster
type nodeMeta struct {
	OverlayAddr net.IPNet
	PubKey      string
}

// Node holds the memberlist node structure
type node struct {
	Name string
	Addr net.IP
	Meta []byte
	nodeMeta
}

func (n *node) String() string {
	return n.Addr.String()
}

func encodeNodeMeta(nm nodeMeta, limit int) []byte {
	buf := &bytes.Buffer{}
	if err := gob.NewEncoder(buf).Encode(nm); err != nil {
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

func parseNodesMeta(rawNodes []node) []node {
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
	return nodes
}
