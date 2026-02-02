module testing/go/fiber

go 1.25.1

require (
	github.com/gofiber/fiber/v2 v2.52.6
	go.tracewayapp.com v0.4.1
	go.tracewayapp.com/tracewayfiber v0.0.0-00010101000000-000000000000
)

require (
	github.com/andybalholm/brotli v1.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.51.0 // indirect
	github.com/valyala/tcplisten v1.0.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
)

replace go.tracewayapp.com => ../../../../go-client

replace go.tracewayapp.com/tracewayfiber => ../../../../go-client/tracewayfiber
