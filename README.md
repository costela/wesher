[![Build Status](https://travis-ci.com/costela/wesher.svg?branch=master)](https://travis-ci.com/costela/wesher)
[![Go Report Card](https://goreportcard.com/badge/github.com/costela/wesher)](https://goreportcard.com/report/github.com/costela/wesher)

# wesher

<img src="./dist/wesher.svg" width="300"/>

`wesher` creates and manages an encrypted mesh overlay network across a group of nodes, using [wireguard](https://www.wireguard.com/).

Its main use-case is adding low-maintenance security to public-cloud networks or connecting different cloud providers.

**⚠ WARNING**: since mesh membership is controlled by a mesh-wide pre-shared key, this effectively downgrades some of the
security benefits from wireguard. See [security considerations](#security-considerations) below for more details.

## Quickstart

0. Before starting:
   1. make sure the [wireguard](https://www.wireguard.com/) kernel module is available on all nodes. It is bundled with linux newer than 5.6 and can otherwise be installed following the instructions [here](https://www.wireguard.com/install/).

   2. The following ports must be accessible between all nodes (see [configuration options](#configuration-options) to change these):
      - 51820 UDP
      - 7946 UDP and TCP

1. Download the latest release for your architecture:

   ```
   $ wget -O wesher https://github.com/costela/wesher/releases/latest/download/wesher-$(go env GOARCH)
   $ chmod a+x wesher
   ```

2. On the first node:
   ```
   # ./wesher
   ```

   This will start the wesher daemon in the foreground and - when running on a terminal - will currently output a generated cluster key as follows:
   ```
   new cluster key generated: XXXXX
   ```

   **Note**: to avoid accidentally leaking it in the logs, the created key will _only_ be displayed if running on a terminal. When started via other means (e.g.: desktop session manager or init system), the key can be retreived with `grep ClusterKey /var/lib/wesher/state.json`.

3. Lastly, on any further node:
   ```
   # wesher --cluster-key XXXXX --join x.x.x.x
   ```

   Where `XXXXX` is the base64 encoded 256 bit key printed by the step above, and `x.x.x.x` is the hostname or IP of any of the nodes already joined to the mesh cluster.

### Permissions 

Note that `wireguard` - and therefore `wesher` - need root access to work properly.

It is also possible to give the `wesher` binary enough capabilities to manage the `wireguard` interface via:
```
# setcap cap_net_admin=eip wesher
```
This will enable running as an unprivileged user, but some functionality (like automatic adding peer entries to
`/etc/hosts`; see [configuration options](#configuration-options) below) will not work.

### (optional) systemd integration

A minimal `systemd` unit file is provided under the `dist` folder and can be copied to `/etc/systemd/system`:
```
# wget -O /etc/systemd/system/wesher.service https://raw.githubusercontent.com/costela/wesher/master/dist/wesher.service
# systemctl daemon-reload
# systemctl enable wesher
```
The provided unit file assumes `wesher` is installed to `/usr/local/sbin`.

Note that, as mentioned above, the initial cluster key will not be displayed in the journal.
It can either be initialized by running `wesher` manually once, or by pre-seeding via `/etc/default/wesher` as the `WESHER_CLUSTER_KEY` environment var (see [configuration options](#configuration-options) below).

## Installing from source

There are a couple of ways of installing `wesher` from sources:

Preferred:
```
$ git clone https://github.com/costela/wesher.git
$ cd wesher
$ make
```
This method can build a bit-by-bit identical binary to the released ones, assuming the same go version is used to build its respective git tag.


Alternatively:
```
$ GO111MODULE=on go get github.com/costela/wesher
```
*Note*: this method will not provide a meaningful output for `--version`.

## Features

The `wesher` tool builds a cluster and manages the configuration of wireguard on each node to create peer-to-peer
connections between all nodes, thus forming a full mesh VPN.
This approach may not scale for hundreds of nodes (benchmarks accepted 😉), but is sufficiently performant to join
several nodes across multiple cloud providers, or simply to secure inter-node comunication in a single public-cloud.

### Automatic Key management

The wireguard private keys are created on startup for each node and the respective public keys are then broadcast
across the cluster.

The control-plane cluster communication is secured with a pre-shared AES-256 key. This key can be be automatically
created during startup of the first node in a cluster, or it can be provided (see [configuration](#configuration-options)).
The cluster key must then be sent to other nodes via a out-of-band secure channel (e.g. ssh, cloud-init, etc).
Once set, the cluster key is saved locally and reused on the next startup.

### Automatic IP address management

The overlay IP address of each node is automatically selected out of a private network (`10.0.0.0/8` by default; MUST be different from the underlying network used for cluster communication) and is consistently hashed based on the peer's hostname.

The use of consistent hashing means a given node will always receive the same overlay IP address (see [limitations](#overlay-ip-collisions)
of this approach below).

**Note**: the node's hostname is also used by the underlying cluster management (using [memberlist](https://github.com/hashicorp/memberlist))
to identify nodes and must therefore be unique in the cluster.

### Automatic /etc/hosts management

To ease intra-node communication, `wesher` also adds entries to `/etc/hosts` for each peer in the mesh. This enables using the nodes' hostnames to ensure communication over the secured overlay network (assuming `files` is the first entry for `hosts` in `/etc/nsswitch.conf`).

See [configuration](#configuration-options) below for how to disable this behavior.

### Seamless restarts

If a node in the cluster is restarted, it will attempt to re-join the last-known nodes using the same cluster key.
This means a restart requires no manual intervention.

## Configuration options

All options can be passed either as command-line flags or environment variables:

| Option | Env | Description | Default |
|---|---|---|---|
| `--cluster-key KEY` | WESHER_CLUSTER_KEY | shared key for cluster membership; must be 32 bytes base64 encoded; will be generated if not provided | autogenerated/loaded |
| `--join HOST,...` | WESHER_JOIN | comma separated list of hostnames or IP addresses to existing cluster members; if not provided, will attempt resuming any known state or otherwise wait for further members |  |
| `--init` | WESHER_INIT | whether to explicitly (re)initialize the cluster; any known state from previous runs will be forgotten | `false` |
| `--bind-addr ADDR` | WESHER_BIND_ADDR | IP address to bind to for cluster membership (cannot be used with --bind-iface) | autodetected |
| `--bind-iface IFACE` | WESHER_BIND_IFACE | Interface to bind to for cluster membership (cannot be used with --bind-addr)|  |
| `--cluster-port PORT` | WESHER_CLUSTER_PORT | port used for membership gossip traffic (both TCP and UDP); must be the same across cluster | `7946` |
| `--wireguard-port PORT` | WESHER_WIREGUARD_PORT | port used for wireguard traffic (UDP); must be the same across cluster | `51820` |
| `--overlay-net ADDR/MASK` | WESHER_OVERLAY_NET | the network in which to allocate addresses for the overlay mesh network (CIDR format); smaller networks increase the chance of IP collision | `10.0.0.0/8` |
| `--interface DEV` | WESHER_INTERFACE | name of the wireguard interface to create and manage | `wgoverlay` |
| `--routed-net NETWORK/CIDR` | WESHER_ROUTED_NET | additional network to be routed to the node on which wesher runs | |
| `--mtu MTU` | WESHER_MTU | MTU value for the wireguard interface | `mtu` |
| `--node-update-script PATH_TO_SCRIPT` | WESHER_NODE_UPDATE_SCRIPT | script to execute everytime there is a node change, this runs as soon as a node joins, updates and/or leaves the cluster. In conjunction with `--routed-net`, which doesn't add routes automatically, this can be used to add routes very flexible depending on each individual system. See utilites/update-node-routes.sh as an example script |  |
| `--no-etc-hosts` | WESHER_NO_ETC_HOSTS | whether to skip writing hosts entries for each node in mesh | `false` |
| `--log-level LEVEL` | WESHER_LOG_LEVEL | set the verbosity (one of debug/info/warn/error) | `warn` |

## Running multiple clusters

To make a node be a member of multiple clusters, simply start multiple wesher instances.  
Each instance **must** have different values for the following settings:
- `--interface`
- either `--cluster-port`, or `--bind-addr` or `--bind-iface`
- `--wireguard-port`

The following settings are not required to be unique, but recommended:
- `--overlay-net` (to reduce the chance of node address conflicts; see [Overlay IP collisions](#overlay-ip-collisions))
- `--cluster-key` (as a sensible security measure)

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
periodically restart `wesher` with the `--join` option pointing to the other side.
Future versions might include the notion of a "static" node to more cleanly avoid this.
