package secret

import (
	"crypto/rand"
	"io/ioutil"
	"os"

	"github.com/libp2p/go-libp2p/core/crypto"
)

func LoadOrCreate(filename string) (crypto.PrivKey, error) {

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return createKey(filename)
		}
		return nil, err
	}

	privateKey, err := crypto.UnmarshalPrivateKey(data)
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

func LoadPsk(filename string) ([]byte, error) {
	return ioutil.ReadFile(filename)
}

func createKey(filename string) (crypto.PrivKey, error) {
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}

	data, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, err
	}

	err = ioutil.WriteFile(filename, data, 0600)
	if err != nil {
		return nil, err
	}

	return priv, nil
}
