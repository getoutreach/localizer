module github.com/jaredallard/localizer

go 1.15

require (
	github.com/cenkalti/backoff/v4 v4.0.2
	github.com/docker/distribution v2.7.1+incompatible
	github.com/golang/protobuf v1.4.2
	github.com/google/uuid v1.1.1
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.6.0
	github.com/txn2/txeh v1.3.0
	github.com/urfave/cli/v2 v2.2.0
	google.golang.org/grpc v1.33.0-dev
	google.golang.org/protobuf v1.25.0
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	k8s.io/kubectl v0.19.2
)

replace k8s.io/client-go => github.com/jaredallard/client-go v0.0.0-20200919203213-e55c7f2b41ab
