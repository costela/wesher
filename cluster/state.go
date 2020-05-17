package cluster

import (
	"encoding/json"
	"fmt"
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

var statePathTemplate = "/var/lib/wesher/%s.json"

const deprecatedStatePath = "/var/lib/wesher/state.json"

func (s *state) save(clusterName string) error {
	statePath := fmt.Sprintf(statePathTemplate, clusterName)
	if err := os.MkdirAll(path.Dir(statePath), 0700); err != nil {
		return err
	}

	stateOut, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(statePath, stateOut, 0600)
}

func loadState(cs *state, clusterName string) {
	statePath := fmt.Sprintf(statePathTemplate, clusterName)
	content, err := ioutil.ReadFile(statePath)
	if err != nil {
		// try the deprecated pre 0.3 state path, it will later
		// be saved to the proper path
		if os.IsNotExist(err) {
			content, err = ioutil.ReadFile(deprecatedStatePath)
		}

		if err != nil {
			if !os.IsNotExist(err) {
				logrus.Warnf("could not open state in %s: %s", statePath, err)
			}
			return
		}
	}

	// avoid partially unmarshalled content by using a temp var
	csTmp := &state{}
	if err := json.Unmarshal(content, csTmp); err != nil {
		logrus.Warnf("could not decode state: %s", err)
	} else {
		*cs = *csTmp
	}
}
