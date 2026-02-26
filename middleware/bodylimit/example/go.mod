module example-body-limit

go 1.25.0

require (
	rivaas.dev/binding v0.0.0
	rivaas.dev/middleware/bodylimit v0.0.0
	rivaas.dev/router v0.11.0
)

require (
	github.com/kr/text v0.2.0 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	rivaas.dev/binding => ../../../binding
	rivaas.dev/middleware/bodylimit => ../
	rivaas.dev/router => ../../../router
)
