module github.com/getoutreach/localizer

go 1.17

require (
	github.com/Microsoft/go-winio v0.5.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d
	github.com/benbjohnson/clock v1.1.0
	github.com/davecgh/go-spew v1.1.1
	github.com/elazarl/goproxy v0.0.0-20210110162100-a92cc753f88e // indirect
	github.com/function61/gokit v0.0.0-20210402130425-341c2c9ecfd0
	github.com/go-logr/logr v0.4.0
	github.com/go-sql-driver/mysql v1.6.0 // indirect
	github.com/google/go-cmp v0.5.6
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/googleapis/gnostic v0.5.5 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/mattn/go-sqlite3 v2.0.3+incompatible // indirect
	github.com/metal-stack/go-ipam v1.8.5
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.1
	github.com/urfave/cli/v2 v2.3.0
	go.opencensus.io v0.23.0 // indirect
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	golang.org/x/oauth2 v0.0.0-20210514164344-f6687ab2804c // indirect
	golang.org/x/sys v0.0.0-20210910150752-751e447fb3d0 // indirect
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20210716133855-ce7ef5c701ea // indirect
	google.golang.org/grpc v1.40.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect

	// kubernetes deps
	k8s.io/api v0.22.4
	k8s.io/apimachinery v0.22.4
	k8s.io/client-go v0.22.4
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a // indirect
)

require (
	cloud.google.com/go v0.65.0 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.12 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.5 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.0 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/avast/retry-go v3.0.0+incompatible // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/form3tech-oss/jwt-go v3.2.2+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/jmoiron/sqlx v1.3.4 // indirect
	github.com/json-iterator/go v1.1.11 // indirect
	github.com/lib/pq v1.10.1 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/russross/blackfriday/v2 v2.0.1 // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go4.org/intern v0.0.0-20210108033219-3eb7198706b2 // indirect
	go4.org/unsafe/assume-no-moving-gc v0.0.0-20201222180813-1025295fd063 // indirect
	golang.org/x/net v0.0.0-20210913180222-943fd674d43e // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	inet.af/netaddr v0.0.0-20210511181906-37180328850c // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.1.2 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace k8s.io/client-go => github.com/jaredallard/client-go v0.21.0-jaredallard
