package otelprocessor

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const processorVersion = "0.1.0"

var componentType = component.MustNewType("source_map_symbolicator")

func NewFactory() processor.Factory {
	return processor.NewFactory(
		componentType,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, component.StabilityLevelAlpha),
		processor.WithLogs(createLogsProcessor, component.StabilityLevelAlpha),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		SymbolicatorFailureAttributeKey:       "exception.symbolicator.failed",
		SymbolicatorErrorAttributeKey:         "exception.symbolicator.error",
		SymbolicatorParsingMethodAttributeKey: "exception.symbolicator.parsing_method",

		ColumnsAttributeKey:   "exception.structured_stacktrace.columns",
		FunctionsAttributeKey: "exception.structured_stacktrace.functions",
		LinesAttributeKey:     "exception.structured_stacktrace.lines",
		UrlsAttributeKey:      "exception.structured_stacktrace.urls",

		StackTraceAttributeKey:       "exception.stacktrace",
		ExceptionTypeAttributeKey:    "exception.type",
		ExceptionMessageAttributeKey: "exception.message",

		PreserveStackTrace:             true,
		OriginalStackTraceAttributeKey: "exception.stacktrace.original",
		OriginalColumnsAttributeKey:    "exception.structured_stacktrace.columns.original",
		OriginalFunctionsAttributeKey:  "exception.structured_stacktrace.functions.original",
		OriginalLinesAttributeKey:      "exception.structured_stacktrace.lines.original",
		OriginalUrlsAttributeKey:       "exception.structured_stacktrace.urls.original",

		BuildUUIDAttributeKey: "app.debug.source_map_uuid",

		SourceMapStoreKey: fileStoreKey,
		LocalSourceMaps:   LocalSourceMapsConfig{Path: "."},

		Timeout: 5 * time.Second,

		SourceMapCacheSize: 128,

		CacheMaxMB: 2048,

		LanguageAttributeKey: "telemetry.sdk.language",
	}
}

func newSymbolicator(cfg *Config, set processor.Settings) (*symbolicatorProcessor, error) {
	store, err := newStore(cfg)
	if err != nil {
		return nil, err
	}
	cache, err := newResolverCache(cfg)
	if err != nil {
		return nil, err
	}
	return &symbolicatorProcessor{
		cfg:    cfg,
		store:  store,
		cache:  cache,
		logger: set.Logger,
	}, nil
}

func createTracesProcessor(ctx context.Context, set processor.Settings, cfg component.Config, next consumer.Traces) (processor.Traces, error) {
	sp, err := newSymbolicator(cfg.(*Config), set)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewTraces(ctx, set, cfg, next, sp.processTraces,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

func createLogsProcessor(ctx context.Context, set processor.Settings, cfg component.Config, next consumer.Logs) (processor.Logs, error) {
	sp, err := newSymbolicator(cfg.(*Config), set)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewLogs(ctx, set, cfg, next, sp.processLogs,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}
