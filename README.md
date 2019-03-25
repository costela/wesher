# wesher

Mesh overlay network manager, using [wireguard](https://www.wireguard.com/).

**⚠ WARNING**: since mesh membership is controlled by a mesh-wide pre-shared key, this effectively downgrades some of the
security benefits from wireguard. See "security considerations" below for more info.

## Quickstart

Before starting, make sure [wireguard](https://www.wireguard.com/) is installed on all nodes.

Install `wesher` on all nodes with:
```
$ go get github.com/costela/wesher
```

On the first node (assuming `$GOPATH/bin` is in the `$PATH`):
```
# wesher
```

Running the command above on a terminal will currently output a generated cluster key, like:
```
new cluster key generated: XXXXX
```

Then, on any further node:
```
# wesher --clusterkey XXXXX --joinaddrs x.x.x.x
```

Where `XXXXX` is the base64 encoded 32 bit key printed by the step above and `x.x.x.x` is the hostname or IP of any of
the nodes already joined to the mesh cluster.

*Note*: `wireguard`, and therefore `wesher`, need root access.

## Overview

## Configuration options

All options can be passed either as command-line flags or environment variables:

| Option | Env | Description | Default |
|---|---|---|---|
| --cluster-key | WESHER_CLUSTER_KEY | shared key for cluster membership; must be 32 bytes base64 encoded; will be generated if not provided |  |
| --join | WESHER_JOIN | comma separated list of hostnames or IP addresses to existing cluster members; if not provided, will attempt resuming any known state or otherwise wait for further members |  |
| --bind-addr | WESHER_BIND_ADDR | IP address to bind to for cluster membership | `0.0.0.0` |
| --cluster-port | WESHER_CLUSTER_PORT | port used for membership gossip traffic (both TCP and UDP); must be the same across cluster | `7946` |
| --wireguard-port | WESHER_WIREGUARD_PORT | port used for wireguard traffic (UDP); must be the same across cluster | `51820` |
| --overlay-net | WESHER_OVERLAY_NET | the network in which to allocate addresses for the overlay mesh network (CIDR format); smaller networks increase the chance of IP collision | `10.0.0.0/8` |
| --interface | WESHER_INTERFACE | name of the wireguard interface to create and manage | `wgoverlay` |
| --log-level | WESHER_LOG_LEVEL | set the verbosity (debug/info/warn/error) | `warn` |


## Security considerations

The decision of whom to allow in the mesh is made by [memberlist](github.com/hashicorp/memberlist) and is secured by a
cluster-wide pre-shared key.
Compromise of this key will allow an attacker to:
- access services exposed on the overlay network
- impersonate and/or disrupt traffic to/from other nodes
It will not, however, allow the attacker access to decrypt the traffic between other nodes.

This pre-shared key is currently static, set up during cluster bootstrapping, but will - in a future version - be
rotated.

## Current known limitations

### Overlay IP collisions

Since the assignment of IPs on the overlay network is currently decided by the individual node and implemented as a
naive hashing of the hostname, there can be no guarantee two hosts will not generate the same overlay IPs.
This limitation may be worked around in a future version.

