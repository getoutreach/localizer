# localizer

A no-frills local development approach for Kubernetes powered Developer Environments.

## Why another CLI tool?

Tools such as; Telepresence, Skaffold, and others all attempt to solve the problem of getting users
used to using Kubernetes. This is a pretty big task given that Kubernetes has a gigantic surface
area. From my experience (**keyword**: _my experience_), developers have no interest in what
platform they are deploying to. All they care about is it's easy to work with and that local development is
not complicated or different from what they are used to. I, also, firmly believe that the best dev-tool is
a tool that requires no configuration, and is self-explanatory. Which these tools tend... to not be.

Given the above, localizer attempts to solve this problem with a few rules:

* A kubernetes cluster should be able to be run locally, but applications should be accessible as if
they were running "locally" (**Note**: Only on Linux do containers _actually_ run locally, the rest are VMs pretending)
* There should be little-to-no DSL to interact with services running in Kubernetes locally.
* No assurances of code working locally will just "work" in Kubernetes. (Let's face it, what you're running locally will never match your production clusters 100%, and if we need to test Kubernetes manifests/etc we should be deploying things into our local cluster and be used to debugging whatever way you do that).

## What does `localizer` actually do?

Given the context of why this was created, and the constraints listed therein, localizer solves these issues
by emulating services running locally. It, essentially, can be considered a fancy wrapper around `kubectl port-forward`
in it's basic operation. Services in a cluster are automatically discovered and tunnels are created. While running
an entry will be added to your local dns resolver that will also allow the service to be looked up by its Kubernetes
service name. This allows a thin emulation layer of Kubernetes, without the caveats of a real Kubernetes cluster.

### Example: PostgreSQL

Let's take an example of having a PostgreSQL pod in the `postgres` namespace, when `localizer` is run the `postgres`
service will be found, port 5432 will be forwarded and accessible via `localhost:5432` and `postgres.postgres[.svc.cluster.local]` will be added to `/etc/hosts`. You could then run `psql` or some other tooling locally and transparently access
resources in your Kubernetes cluster as if they were running outside of the cluster.

### Example: Letting services inside of Kubernetes talk to a local service

When running `localizer expose <serviceName>` your local machine will look for an existing service in your
Kubernetes cluster, and if it exists it will create a container that will proxy traffic sent to it to your local machine
allowing remote resources to access your local machine as if they were also running locally.

## Install `localizer`

You can install the (OSX/LINUX) binary directly into /usr/local/bin:
```
$ curl -Ls https://github.com/getoutreach/localizer/releases/download/v1.9.0/localizer_1.9.0_$(uname)_amd64.tar.gz | tar -xzC /usr/local/bin localizer
```

Or manually download a release from [Github Releases](../../releases/latest) and unpack it into your `PATH`.

## How do I run `localizer`?

Easy, just run the following:

```
$ sudo -E localizer
```

This will attempt to proxy all services in Kubernetes to your local machine under their respective ports.

## FAQ

### Does `localizer` support Windows?

WSL2 should work, and I'd consider it supported. I wrote most of this on WSL2, but I will likely maintain it on `macOS`.
Outside of WSL? Not currently. PRs are welcome!

## License

Apache-2.0
