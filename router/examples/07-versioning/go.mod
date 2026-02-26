module example-07-versioning

go 1.25.0

require rivaas.dev/router v0.0.0

require (
	github.com/kr/pretty v0.3.1 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	rivaas.dev/binding => ../../../binding
	rivaas.dev/router => ../../
)
