module testing/go/chi

go 1.25.1

require (
	github.com/go-chi/chi/v5 v5.2.1
	go.tracewayapp.com v0.4.1
	go.tracewayapp.com/tracewaychi v0.0.0-00010101000000-000000000000
)

require (
	github.com/google/uuid v1.6.0 // indirect
	go.tracewayapp.com/tracewayhttp v0.4.1 // indirect
	golang.org/x/sys v0.35.0 // indirect
)

replace go.tracewayapp.com => ../../../../go-client

replace go.tracewayapp.com/tracewaychi => ../../../../go-client/tracewaychi

replace go.tracewayapp.com/tracewayhttp => ../../../../go-client/tracewayhttp
