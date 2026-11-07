module gorex

go 1.27.3

require (
	github.com/ebitengine/purego v0.11.1
	github.com/egoist/mygo v0.3.5-0.20261008155018-bad73d119dc8
	golang.org/x/image v0.46.0
	golang.org/x/sys v0.48.0
)

require (
	github.com/go-text/typesetting v0.3.5 // indirect
	golang.org/x/net v0.59.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)

tool github.com/egoist/mygo/cmd/mygo

replace github.com/egoist/mygo => ../../egoist/mygo
