module example-request-id

go 1.25.0

require (
	rivaas.dev/middleware/accesslog v0.0.0
	rivaas.dev/middleware/requestid v0.0.0
	rivaas.dev/router v0.11.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/oklog/ulid/v2 v2.1.1 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	rivaas.dev/binding => ../../../../binding
	rivaas.dev/logging => ../../../../logging
	rivaas.dev/middleware/accesslog => ../../accesslog
	rivaas.dev/middleware/recovery => ../../recovery
	rivaas.dev/middleware/requestid => ../../requestid
	rivaas.dev/router => ../../../router
)
