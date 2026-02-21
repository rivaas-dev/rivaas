module example-04-middleware-stack

go 1.25.0

require (
	github.com/charmbracelet/log v0.4.2
	rivaas.dev/middleware/accesslog v0.0.0
	rivaas.dev/middleware/cors v0.0.0
	rivaas.dev/middleware/recovery v0.0.0
	rivaas.dev/middleware/timeout v0.0.0
	rivaas.dev/router v0.10.0
)

require (
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/charmbracelet/colorprofile v0.4.2 // indirect
	github.com/charmbracelet/lipgloss v1.1.0 // indirect
	github.com/charmbracelet/x/ansi v0.11.6 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.15 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.10.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/go-logfmt/logfmt v0.6.1 // indirect
	github.com/lucasb-eyer/go-colorful v1.3.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.20 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	go.opentelemetry.io/otel v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.40.0 // indirect
	golang.org/x/exp v0.0.0-20260218203240-3dfff04db8fa // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/term v0.40.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	rivaas.dev/binding => ../../../binding
	rivaas.dev/logging => ../../../logging
	rivaas.dev/middleware/accesslog => ../../../middleware/accesslog
	rivaas.dev/middleware/cors => ../../../middleware/cors
	rivaas.dev/middleware/recovery => ../../../middleware/recovery
	rivaas.dev/middleware/requestid => ../../../middleware/requestid
	rivaas.dev/middleware/timeout => ../../../middleware/timeout
	rivaas.dev/router => ../../
)
