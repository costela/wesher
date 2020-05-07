package common

import (
	"bytes"
	"encoding/gob"
	"net"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// NodeMeta holds metadata sent over the cluster
type NodeMeta struct {
	OverlayAddr net.IPNet
	PubKey      string
}

// Node holds the memberlist node structure
type Node struct {
	Name string
	Addr net.IP
	Meta []byte
	NodeMeta
}

func (n *Node) String() string {
	return n.Addr.String()
}

func EncodeNodeMeta(nm NodeMeta, limit int) []byte {
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

func DecodeNodeMeta(b []byte) (NodeMeta, error) {
	// TODO: we blindly trust the info we get from the peers; We should be more defensive to limit the damage a leaked
	// PSK can cause.
	nm := NodeMeta{}
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&nm); err != nil {
		return nm, errors.Wrap(err, "could not decode node meta")
	}
	return nm, nil
}
