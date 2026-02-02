module testing/go/fasthttp

go 1.25.1

require (
	github.com/fasthttp/router v1.5.4
	github.com/valyala/fasthttp v1.62.0
	go.tracewayapp.com v0.4.1
	go.tracewayapp.com/tracewayfasthttp v0.0.0-00010101000000-000000000000
)

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/savsgio/gotils v0.0.0-20240704082632-aef3928b8a38 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
)

replace go.tracewayapp.com => ../../../../go-client

replace go.tracewayapp.com/tracewayfasthttp => ../../../../go-client/tracewayfasthttp
