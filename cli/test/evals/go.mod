module github.com/tracewayapp/traceway/cli/test/evals

go 1.26.2

toolchain go1.26.11

require (
	github.com/benhoyt/goawk v1.32.0
	github.com/google/uuid v1.6.0
	github.com/itchyny/gojq v0.12.19
	github.com/modelcontextprotocol/go-sdk v1.6.1
	go.opentelemetry.io/proto/otlp v1.10.0
	golang.org/x/term v0.45.0
	google.golang.org/protobuf v1.36.11
	gopkg.in/yaml.v3 v3.0.1
	mvdan.cc/sh/v3 v3.14.1
)

require (
	github.com/google/jsonschema-go v0.4.3 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.28.0 // indirect
	github.com/itchyny/timefmt-go v0.1.8 // indirect
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/segmentio/encoding v0.5.4 // indirect
	github.com/tracewayapp/traceway/cli/v2 v2.0.11
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/oauth2 v0.35.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260209200024-4cfbd4190f57 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260209200024-4cfbd4190f57 // indirect
	google.golang.org/grpc v1.79.2 // indirect
)

replace github.com/tracewayapp/traceway/cli/v2 => ../..
