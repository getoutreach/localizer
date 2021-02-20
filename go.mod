module github.com/jaredallard/localizer

go 1.15

require (
	github.com/asaskevich/govalidator v0.0.0-20200907205600-7a23bdc65eef
	github.com/benbjohnson/clock v1.1.0
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/function61/gokit v0.0.0-20201222133023-aeb11a5badac
	github.com/go-logr/logr v0.3.0
	github.com/golang/protobuf v1.4.3
	github.com/google/go-cmp v0.5.4
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/metal-stack/go-ipam v1.7.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.0
	github.com/urfave/cli/v2 v2.3.0
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83
	google.golang.org/grpc v1.35.0
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v3 v3.0.0-20200605160147-a5ece683394c // indirect
	gotest.tools v2.2.0+incompatible // indirect

	// kubernetes deps
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.0.0-00010101000000-000000000000
	k8s.io/klog/v2 v2.4.0
)

replace (
	// This fixes macOS builds for now.
	golang.org/x/sys => golang.org/x/sys v0.0.0-20200826173525-f9321e4c35a6
	k8s.io/client-go => github.com/jaredallard/client-go v0.0.0-20201216203657-cc48b6b74693
)
