module github.com/jaredallard/localizer

go 1.15

require (
	github.com/cenkalti/backoff/v4 v4.1.0
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/function61/gokit v0.0.0-20200923114939-f8d7e065a5c3
	github.com/go-logr/logr v0.3.0
	github.com/golang/protobuf v1.4.3
	github.com/imdario/mergo v0.3.8
	github.com/metal-stack/go-ipam v1.7.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.0
	github.com/txn2/txeh v1.3.0
	github.com/urfave/cli/v2 v2.3.0
	golang.org/x/crypto v0.0.0-20201112155050-0c6587e931a9
	google.golang.org/grpc v1.33.2
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v3 v3.0.0-20200605160147-a5ece683394c // indirect

	// kubernetes deps
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.19.3
	k8s.io/client-go v0.19.3
	k8s.io/klog/v2 v2.4.0
	k8s.io/kubectl v0.19.3
)

replace k8s.io/client-go => github.com/jaredallard/client-go v0.0.0-20200919203213-e55c7f2b41ab

// This fixes macOS builds for now.
replace golang.org/x/sys => golang.org/x/sys v0.0.0-20200826173525-f9321e4c35a6
