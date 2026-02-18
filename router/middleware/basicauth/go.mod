module rivaas.dev/router/middleware/basicauth

go 1.25

require (
	github.com/onsi/ginkgo/v2 v2.28.1
	github.com/onsi/gomega v1.39.1
	github.com/stretchr/testify v1.11.1
	rivaas.dev/router v0.0.0
	rivaas.dev/router/middleware/accesslog v0.0.0
	rivaas.dev/router/middleware/cors v0.0.0
	rivaas.dev/router/middleware/recovery v0.0.0
	rivaas.dev/router/middleware/requestid v0.0.0
	rivaas.dev/router/middleware/security v0.0.0
)

require (
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20260202012954-cb029daf43ef // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/oklog/ulid/v2 v2.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.opentelemetry.io/otel v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.40.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.33.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/term v0.40.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	golang.org/x/tools v0.42.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	rivaas.dev/router => ../../
	rivaas.dev/router/middleware/accesslog => ../accesslog
	rivaas.dev/router/middleware/cors => ../cors
	rivaas.dev/router/middleware/recovery => ../recovery
	rivaas.dev/router/middleware/requestid => ../requestid
	rivaas.dev/router/middleware/security => ../security
)
