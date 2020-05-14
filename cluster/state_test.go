package cluster

import (
	"net"
	"reflect"
	"testing"

	"github.com/costela/wesher/common"
)

func Test_state_save_soad(t *testing.T) {
	statePath := "/tmp/wesher.json"
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

	if err := cluster.state.save(statePath); err != nil {
		t.Error(err)
	}
	loaded := &state{}
	loadState(loaded, statePath)

	if !reflect.DeepEqual(cluster.state, loaded) {
		t.Errorf("cluster state save then reload mistmatch: %s / %s", cluster.state, loaded)
	}
}
