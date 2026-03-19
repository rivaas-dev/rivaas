module example-security

go 1.25.0

require (
	rivaas.dev/middleware/security v0.0.0
	rivaas.dev/router v0.15.0
)

require (
	github.com/kr/text v0.2.0 // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	rivaas.dev/binding => ../../../../binding
	rivaas.dev/middleware/security => ../../security
	rivaas.dev/router => ../../../router
)
