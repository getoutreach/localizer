# Introduction / Scope

Localizer was made to address the gap between local machines and their local Kubernetes clusters. Due to multi-platform Kubernetes clusters usually running inside of a VM, there are network boundaries, among other things, between the developer and their cluster. Localizer solves this problem by being a cross between a VPN and an SSH reverse proxy. This breaks localizer into two distinct areas:

 * VPN-like implementation (proxier/tunnel)
 * SSH Reverse Proxy (expose)

These two areas are held together and controlled by a GRPC server to allow Localizer to run as a daemon. Subsequent calls to localizer work as a client to that daemon.

# Packages

Among the two features of Localizer, tunnel and expose, there are a bunch of different packages that make up Localizer:

 * `expose` - Handles creating an SSH-powered reverse proxy from the k8s cluster to the local machine
 * `kube` - Kubernetes client and other functions
 * `kevents` - Kubernetes global cache
 * `proxier` - Kubernetes port-forward manager, the VPN-like implementation 
 * `server` - GRPC server implementation for the daemon
 * `ssh` - Implementation of an SSH client + reverse proxy

Outside of these packages, there is the CLI layer that "glues" all of this together.

## CLI Design

Localizer has a pretty minimal design for the CLI layer, which lives inside of the cmd directory and uses urfave/cli to implement commands. In general, the principle behind each of these command implementations (separated by name.go) is that they should implement mostly bare minimum formatting or user input logic only. All other logic should be located in a separate package.

All other Localizer commands, such as expose and list, are implemented by talking to the GRPC server.

## GRPC Setup

The CLI is a thin wrapper around two components; the GRPC server and the client. The server is used for the localizer daemon, also starts the Kubernetes VPN (or port-forward manager) aspect of Localizer. This is done by creating a UNIX socket that lives at /var/run/localizer.sock. All of the logic for the GPRC server lives inside of the server package.

The logic for connecting to the server (the client) currently lives in the CLI library, which will eventually be pulled into its own package.

# Kubernetes Tunnels

When the GRPC server is started, it starts running our Kubernetes VPN, or port-forward manager. This is done (thanks to @databus23!) by using client-go's SharedInformer and work queue libraries. This is much like an [operator-sdk](https://github.com/operator-framework/operator-sdk) generated operator. Localizer works by fetching a list of services and using a work queue to process them. When a service is processed, a `kubectl port-forward` (essentially) is created allowing access to that service. In order to mitigate port collisions, this is done by listening on a virtual IP address. On Darwin, this is done by creating an IP alias. On Linux/WSL, this is done by listening on a 127.X.X.X address.

These tunnels are refreshed by that same work queue, when a service is deleted, the subsequent tunnel is deleted and no longer tracked. When an endpoint is removed, that a tunnel is powered by, it is recreated with a new endpoint or backed off until one is created.

# Hosts Library

When a tunnel has allocated an IP address, there is still a missing component that Kubernetes provides to pods: DNS. In order to facilitate supporting DNS resolution outside of the cluster, Localizer modifies the local machine's `/etc/hosts` file to point to its IP address. This is done by the library in `pkg/hostsfile`. This library works by allocating a "block", wrapped in comments, that it will write to. Everything outside of this block is not touched and left alone. This reduces the invasiveness of changes to this file.

# Expose Tunnels

The codebase for expose is entirely different from the rest of the application, except the GRPC server is still the entry point. When Localizer receives a request asking for a reverse tunnel (e.g. expose is ran), Localizer does two things. It first looks up the service, if it doesn't exist it returns an error. If it exists, it looks for all endpoints on that service. This allows Localizer to be forward compatible with any new object types that Kubernetes may introduce since Kubernetes only routes traffic to endpoints. For each endpoint found, it attempts to look up what type of object it is. If it's a Pod, it'll look for a replica set. If the pod has no `ReplicaSet` attached, it'll ignore it. This is because there is no way to safely scale down this pod without deleting it forever. An error is logged in this case. If a `ReplicaSet` is found, then the parent object is looked up. This object is then scaled down to 0. The generic logic allows us to scale down `Deployment` and `StatefulSet` the same way.

Once the existing objects have been scaled down, Localizer creates a pod with the name `localizer-<serviceName>` with the exact labels needed by the service to route traffic to it. This pod contains a OpenSSH server docker image listening on port 2222 with static credentials. Localizer then creates a Kubernetes port-forward that exposes this service locally on a random port on the 127.0.0.1 IP. Localizer then creates a reverse tunnel over this Kubernetes port-forward. The end result is that when a service tries to talk to our "localized" service, their traffic is sent to the local service instead. This also works out of the box for tunnels created by Localizer since the pod is just another endpoint.
