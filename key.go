package main

import (
	"encoding"
	"encoding/base64"
)

type key struct {
	bytes []byte
}

var _ encoding.TextUnmarshaler = (*key)(nil)

func (k *key) UnmarshalText(in []byte) error {
	k.bytes = make([]byte, base64.StdEncoding.DecodedLen(len(in)))
	n, err := base64.StdEncoding.Decode(k.bytes, in)
	k.bytes = k.bytes[:n]
	return err
}
