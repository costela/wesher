package cluster

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"

	"github.com/costela/wesher/common"
	"github.com/sirupsen/logrus"
)

// State keeps track of information needed to rejoin the cluster
type state struct {
	ClusterKey []byte
	Nodes      []common.Node
}

const statePath = "/var/lib/wesher/state.json"

func (c *Cluster) saveState() error {
	if err := os.MkdirAll(path.Dir(statePath), 0700); err != nil {
		return err
	}

	stateOut, err := json.MarshalIndent(c.state, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(statePath, stateOut, 0600)
}

func loadState(cs *state) {
	content, err := ioutil.ReadFile(statePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logrus.Warnf("could not open state in %s: %s", statePath, err)
		}
		return
	}

	// avoid partially unmarshalled content by using a temp var
	csTmp := &state{}
	if err := json.Unmarshal(content, csTmp); err != nil {
		logrus.Warnf("could not decode state: %s", err)
	} else {
		*cs = *csTmp
	}
}
