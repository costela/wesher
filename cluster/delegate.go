package cluster

import (
	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"
)

// NotifyConflict implements the memberlist deletage interface
func (c *Cluster) NotifyConflict(node, other *memberlist.Node) {
	logrus.Errorf("node name conflict detected: %s", other.Name)
}

// NodeMeta implements the memberlist deletage interface
// Metadata is provided by the local node settings, encoding is handled
// by the node implementation directly
func (c *Cluster) NodeMeta(limit int) []byte {
	encoded, err := c.localNode.Encode(limit)
	if err != nil {
		logrus.Errorf("failed to encode local node: %s", err)
		return nil
	}
	return encoded
}

// NotifyMsg implements the memberlist deletage interface
func (c *Cluster) NotifyMsg([]byte) {}

// GetBroadcasts implements the memberlist deletage interface
func (c *Cluster) GetBroadcasts(overhead, limit int) [][]byte { return nil }

// LocalState implements the memberlist deletage interface
func (c *Cluster) LocalState(join bool) []byte { return nil }

// MergeRemoteState implements the memberlist deletage interface
func (c *Cluster) MergeRemoteState(buf []byte, join bool) {}
