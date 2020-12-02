module github.com/jaredallard/localizer

go 1.15

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/function61/gokit v0.0.0-20200923114939-f8d7e065a5c3
	github.com/go-logr/logr v0.3.0
	github.com/golang/protobuf v1.4.3
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/metal-stack/go-ipam v1.7.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.0
	github.com/txn2/txeh v1.3.0
	github.com/urfave/cli/v2 v2.3.0
	golang.org/x/crypto v0.0.0-20201124201722-c8d3bf9c5392
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	google.golang.org/grpc v1.33.2
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v3 v3.0.0-20200605160147-a5ece683394c // indirect
	gotest.tools v2.2.0+incompatible // indirect

	// kubernetes deps
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.19.3
	k8s.io/client-go v0.19.3
	k8s.io/klog/v2 v2.4.0
)

replace k8s.io/client-go => github.com/jaredallard/client-go v0.0.0-20200919203213-e55c7f2b41ab

// This fixes macOS builds for now.
replace golang.org/x/sys => golang.org/x/sys v0.0.0-20200826173525-f9321e4c35a6
