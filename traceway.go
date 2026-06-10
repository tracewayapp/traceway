package traceway

import (
	"github.com/tracewayapp/traceway/internal/sourcemapprocessor"
	"go.opentelemetry.io/collector/processor"
)

func SymbolicatorProcessor() processor.Factory {
	return sourcemapprocessor.NewFactory()
}

func NewFactory() processor.Factory {
	return SymbolicatorProcessor()
}
