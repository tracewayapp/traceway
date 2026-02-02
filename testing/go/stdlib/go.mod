module testing/go/stdlib

go 1.25.1

require (
	go.tracewayapp.com v0.4.1
	go.tracewayapp.com/tracewayhttp v0.0.0-00010101000000-000000000000
)

require (
	github.com/google/uuid v1.6.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
)

replace go.tracewayapp.com => ../../../../go-client

replace go.tracewayapp.com/tracewayhttp => ../../../../go-client/tracewayhttp
