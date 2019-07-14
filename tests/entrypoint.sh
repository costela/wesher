#!/bin/sh

set -e

mkdir -p /dev/net
mknod /dev/net/tun c 10 200

wireguard-go wgoverlay

/app/wesher --log-level debug --cluster-key 'ILICZ3yBMCGAWNIq5Pn0bewBVimW3Q2yRVJ/Be+b1Uc=' "$@"