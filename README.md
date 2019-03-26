[![Build Status](https://travis-ci.org/costela/wesher.svg?branch=master)](https://travis-ci.org/costela/wesher)
[![Go Report Card](https://goreportcard.com/badge/github.com/costela/wesher)](https://goreportcard.com/report/github.com/costela/wesher)

# wesher

`wesher` creates and manages a mesh overlay network across a group of nodes, using [wireguard](https://www.wireguard.com/).

Its main use-case is adding low-maintenance security to public-cloud networks or connecting different cloud providers.

**⚠ WARNING**: since mesh membership is controlled by a mesh-wide pre-shared key, this effectively downgrades some of the
security benefits from wireguard. See [security considerations](#security-considerations) below for more details.

## Quickstart

Before starting, make sure [wireguard](https://www.wireguard.com/) is installed on all nodes.

The following ports must be accessible between all nodes (see [configuration options](#configuration-options) to change these):
- 51820 UDP
- 7946 UDP and TCP

Install `wesher` on all nodes with go >= 1.11:
```
$ GO111MODULE=on go get github.com/costela/wesher
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

*Note*: `wireguard` - and therefore `wesher` - need root access.

## Features

The `wesher` tool builds a cluster and manages the configuration of wireguard on each node to create peer-to-peer
connections between all nodes, thus forming a full mesh VPN.
This approach may not scale for hundreds of nodes (benchmarks accepted 😉), but is sufficiently performant to join
several nodes across multiple cloud providers, or simply to secure inter-node comunication in a single public-cloud.

### Automatic Key management

The wireguard private keys are created on startup for each node and the respective public keys are then broadcast
across the cluster. 

The control-plane cluster communication is secured with a pre-shared AES-256 bit key. This key can be be automatically
created during startup of the first node in a cluster, or it can be provided (see [configuration](#configuration-options)).
The cluster key must then be sent to other nodes via a out-of-band secure channel (e.g. ssh, cloud-init, etc).
Once set, the cluster key is saved locally and reused on the next startup.

### Automatic IP address management

The overlay IP address of each node is selected out of a private network (`10.0.0.0/8` by default) and is consistently
hashed based on the hostname, meaning a host will always receive the same overlay IP address (see [limitations](#overlay-ip-collisions)
of this approach below). The hostname is also used by the underlying cluster management (using [memberlist](https://github.com/hashicorp/memberlist))
to identify nodes and must therefore be unique in the cluster.

To ease intra-node communication, `wesher` also adds entries to `/etc/hosts` for each other node. See [configuration](#configuration-options)
below for how to disable this behavior.

### Restoring state

If a node in the cluster is restarted, it will attempt to re-join the last-known nodes using the same cluster key.
This means a restart requires no manual intervention.

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

The decision of whom to allow in the mesh is made by [memberlist](https://github.com/hashicorp/memberlist) and is secured by a
cluster-wide pre-shared key.
Compromise of this key will allow an attacker to:
- access services exposed on the overlay network
- impersonate and/or disrupt traffic to/from other nodes
It will not, however, allow the attacker access to decrypt the traffic between other nodes.

This pre-shared key is currently static, set up during cluster bootstrapping, but will - in a future version - be
rotated for improved security.

## Current known limitations

### Overlay IP collisions

Since the assignment of IPs on the overlay network is currently decided by the individual node and implemented as a
naive hashing of the hostname, there can be no guarantee two hosts will not generate the same overlay IPs.
This limitation may be worked around in a future version.

### Split-brain

Once a cluster is joined, there is currently no way to distinguish a failed node from an intentionally removed one.
This is partially by design: growing and shrinking your cluster dynamically (e.g. via autoscaling) should be as easy
as possible.

However, this does mean longer connection loss between any two parts of the cluster (e.g. across a WAN link between
different cloud providers) can lead to a split-brain scenario where each side thinks the other side is simply "gone".

There is currently no clean solution for this problem, but one could work around it by designating edge nodes which
periodically restart `wesher` with the `--joinaddrs` option pointing to the other side.
Future versions might include the notion of a "static" node to more cleanly avoid this.
