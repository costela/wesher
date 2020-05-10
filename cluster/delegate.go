package cluster

import (
	"github.com/costela/wesher/common"
	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"
)

// DelegateNode implements the memberlist delegation interface
type delegateNode struct {
	*common.Node
}

// NotifyConflict implements the memberlist.Delegate interface
func (n *delegateNode) NotifyConflict(node, other *memberlist.Node) {
	logrus.Errorf("node name conflict detected: %s", other.Name)
}

// NodeMeta implements the memberlist.Delegate interface
// Metadata is provided by the local node settings, encoding is handled
// by the node implementation directly
func (n *delegateNode) NodeMeta(limit int) []byte {
	encoded, err := n.Encode(limit)
	if err != nil {
		logrus.Errorf("failed to encode local node: %s", err)
		return nil
	}
	return encoded
}

// NotifyMsg implements the memberlist.Delegate interface
func (n *delegateNode) NotifyMsg([]byte) {}

// GetBroadcasts implements the memberlist.Delegate interface
func (n *delegateNode) GetBroadcasts(overhead, limit int) [][]byte { return nil }

// LocalState implements the memberlist.Delegate interface
func (n *delegateNode) LocalState(join bool) []byte { return nil }

// MergeRemoteState implements the memberlist.Delegate interface
func (n *delegateNode) MergeRemoteState(buf []byte, join bool) {}
