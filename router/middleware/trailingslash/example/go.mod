module example-trailing-slash

go 1.25.0

require (
	rivaas.dev/router v0.10.0
	rivaas.dev/router/middleware/trailingslash v0.0.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	go.opentelemetry.io/otel v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.40.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	rivaas.dev/binding => ../../../../binding
	rivaas.dev/router => ../../..
	rivaas.dev/router/middleware/trailingslash => ..
)
