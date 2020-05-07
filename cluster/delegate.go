package cluster

import (
	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"
)

func (c *Cluster) NotifyConflict(node, other *memberlist.Node) {
	logrus.Errorf("node name conflict detected: %s", other.Name)
}

func (c *Cluster) NodeMeta(limit int) []byte {
	encoded, err := c.localNode.Encode(limit)
	if err != nil {
		logrus.Errorf("failed to encode local node: %s", err)
		return nil
	}
	return encoded
}

// none of these are used
func (c *Cluster) NotifyMsg([]byte)                           {}
func (c *Cluster) GetBroadcasts(overhead, limit int) [][]byte { return nil }
func (c *Cluster) LocalState(join bool) []byte                { return nil }
func (c *Cluster) MergeRemoteState(buf []byte, join bool)     {}
