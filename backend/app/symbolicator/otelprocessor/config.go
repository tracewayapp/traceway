package otelprocessor

import (
	"fmt"
	"time"
)

type LocalSourceMapsConfig struct {
	Path string `mapstructure:"path"`
}

type S3SourceMapsConfig struct {
	Region string `mapstructure:"region"`
	Bucket string `mapstructure:"bucket"`
	Prefix string `mapstructure:"prefix"`
}

type GCSSourceMapsConfig struct {
	Bucket string `mapstructure:"bucket"`
	Prefix string `mapstructure:"prefix"`
}

type Config struct {
	SymbolicatorFailureAttributeKey       string `mapstructure:"symbolicator_failure_attribute_key"`
	SymbolicatorErrorAttributeKey         string `mapstructure:"symbolicator_error_attribute_key"`
	SymbolicatorParsingMethodAttributeKey string `mapstructure:"symbolicator_parsing_method_attribute_key"`

	ColumnsAttributeKey   string `mapstructure:"columns_attribute_key"`
	FunctionsAttributeKey string `mapstructure:"functions_attribute_key"`
	LinesAttributeKey     string `mapstructure:"lines_attribute_key"`
	UrlsAttributeKey      string `mapstructure:"urls_attribute_key"`

	StackTraceAttributeKey       string `mapstructure:"stack_trace_attribute_key"`
	ExceptionTypeAttributeKey    string `mapstructure:"exception_type_attribute_key"`
	ExceptionMessageAttributeKey string `mapstructure:"exception_message_attribute_key"`

	PreserveStackTrace             bool   `mapstructure:"preserve_stack_trace"`
	OriginalStackTraceAttributeKey string `mapstructure:"original_stack_trace_attribute_key"`
	OriginalColumnsAttributeKey    string `mapstructure:"original_columns_attribute_key"`
	OriginalFunctionsAttributeKey  string `mapstructure:"original_functions_attribute_key"`
	OriginalLinesAttributeKey      string `mapstructure:"original_lines_attribute_key"`
	OriginalUrlsAttributeKey       string `mapstructure:"original_urls_attribute_key"`

	BuildUUIDAttributeKey string `mapstructure:"build_uuid_attribute_key"`

	IOSBuildUUIDAttributeKey  string `mapstructure:"ios_build_uuid_attribute_key"`
	AppExecutableAttributeKey string `mapstructure:"app_executable_attribute_key"`

	SourceMapStoreKey string                `mapstructure:"source_map_store"`
	LocalSourceMaps   LocalSourceMapsConfig `mapstructure:"local_source_maps"`
	S3SourceMaps      S3SourceMapsConfig    `mapstructure:"s3_source_maps"`
	GCSSourceMaps     GCSSourceMapsConfig   `mapstructure:"gcs_source_maps"`

	Timeout time.Duration `mapstructure:"timeout"`

	SourceMapCacheSize int `mapstructure:"source_map_cache_size"`

	CacheDir        string `mapstructure:"cache_dir"`
	CacheMaxMB      int    `mapstructure:"cache_max_mb"`
	CacheMaxDiskPct int    `mapstructure:"cache_max_disk_pct"`

	DartDefaultArch string `mapstructure:"dart_default_arch"`
	IOSDefaultArch  string `mapstructure:"ios_default_arch"`

	LanguageAttributeKey string   `mapstructure:"language_attribute_key"`
	AllowedLanguages     []string `mapstructure:"allowed_languages"`

	Parser string `mapstructure:"parser"`
}

func (c *Config) Validate() error {
	switch c.SourceMapStoreKey {
	case fileStoreKey, s3StoreKey, gcsStoreKey:
	default:
		return fmt.Errorf("unknown source_map_store %q (available: %s, %s, %s)", c.SourceMapStoreKey, fileStoreKey, s3StoreKey, gcsStoreKey)
	}
	if c.CacheMaxDiskPct < 0 || c.CacheMaxDiskPct > 100 {
		return fmt.Errorf("cache_max_disk_pct must be between 0 and 100, got %d", c.CacheMaxDiskPct)
	}
	if c.CacheMaxMB < 0 {
		return fmt.Errorf("cache_max_mb must not be negative, got %d", c.CacheMaxMB)
	}
	if c.SourceMapCacheSize <= 0 {
		return fmt.Errorf("source_map_cache_size must be positive, got %d", c.SourceMapCacheSize)
	}
	return nil
}
