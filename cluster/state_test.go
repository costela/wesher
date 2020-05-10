package cluster

import (
	"net"
	"reflect"
	"testing"

	"github.com/costela/wesher/common"
)

func Test_state_save_soad(t *testing.T) {
	key := "abcdefghijklmnopqrstuvwxyzABCDEF"
	node := common.Node{
		Name: "node",
		Addr: net.ParseIP("10.0.0.2"),
	}

	cluster := Cluster{
		state: &state{
			ClusterKey: []byte(key),
			Nodes:      []common.Node{node},
		},
	}

	cluster.saveState()
	loaded := &state{}
	loadState(loaded)

	if !reflect.DeepEqual(cluster.state, loaded) {
		t.Errorf("cluster state save then reload mistmatch: %s / %s", cluster.state, loaded)
	}
}
