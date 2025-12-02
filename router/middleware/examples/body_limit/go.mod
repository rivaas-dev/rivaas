module example-body-limit

go 1.25.0

require (
	rivaas.dev/binding v0.0.0
	rivaas.dev/router v0.0.0
)

require (
	go.opentelemetry.io/otel v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	rivaas.dev/binding => ../../../../binding
	rivaas.dev/router => ../../../
)
