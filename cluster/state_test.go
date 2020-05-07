package cluster

import (
	"net"
	"reflect"
	"testing"

	"github.com/costela/wesher/common"
)

func Test_State_Save_Load(t *testing.T) {
	key := "abcdefghijklmnopqrstuvwxyzABCDEF"
	node := common.Node{
		Name: "node",
		Addr: net.ParseIP("10.0.0.2"),
	}

	cluster := Cluster{
		state: &State{
			ClusterKey: []byte(key),
			Nodes:      []common.Node{node},
		},
	}

	cluster.saveState()
	loaded := &State{}
	loadState(loaded)

	if !reflect.DeepEqual(cluster.state, loaded) {
		t.Errorf("cluster state save then reload mistmatch: %s / %s", cluster.state, loaded)
	}
}
