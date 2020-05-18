#!/bin/bash

set -e

# Parse arguments
args=("$@")
while [[ $# -gt 0 ]]; do case $1 in
    --interface)
    iface=$2
    shift
    ;;
esac; shift; done

# Create tun device if necessary
if [ ! -e /dev/net/tun ]; then
    mkdir -p /dev/net
    mknod /dev/net/tun c 10 200
fi

wireguard-go ${iface:-wgoverlay}
/app/wesher --log-level debug --cluster-key 'ILICZ3yBMCGAWNIq5Pn0bewBVimW3Q2yRVJ/Be+b1Uc=' "${args[@]}"