package common

import (
	"bytes"
	"encoding/gob"
	"net"

	"github.com/pkg/errors"
)

// nodeMeta holds metadata sent over the cluster
type nodeMeta struct {
	OverlayAddr net.IPNet
	PubKey      string
}

// Node holds the memberlist node structure
type Node struct {
	Name string
	Addr net.IP
	Meta []byte
	nodeMeta
}

func (n *Node) String() string {
	return n.Addr.String()
}

// Encode the node metadata to bytes, in a deterministic reversible way
func (n *Node) Encode(limit int) ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := gob.NewEncoder(buf).Encode(n.nodeMeta); err != nil {
		return nil, errors.Wrap(err, "could not encode local state")
	}
	if buf.Len() > limit {
		return nil, errors.Errorf("could not fit node metadata into %d bytes", limit)
	}
	return buf.Bytes(), nil
}

// Decode the node Meta field into its metadata
func (n *Node) Decode() error {
	// TODO: we blindly trust the info we get from the peers; We should be more defensive to limit the damage a leaked
	// PSK can cause.
	nm := nodeMeta{}
	if err := gob.NewDecoder(bytes.NewReader(n.Meta)).Decode(&nm); err != nil {
		return errors.Wrap(err, "could not decode node meta")
	}
	n.nodeMeta = nm
	return nil
}
