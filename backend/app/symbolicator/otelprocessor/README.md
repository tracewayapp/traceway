# Traceway Source Map Symbolicator Processor

An OpenTelemetry Collector processor that symbolicates minified JavaScript stack traces against your uploaded source maps, powered by Traceway's symbolicator engine. It is a drop-in replacement for [Honeycomb's `source_map_symbolicator` processor](https://github.com/honeycombio/opentelemetry-collector-symbolicator): same component type, same attribute contract, same store layout, same configuration keys. Existing pipelines and instrumentation (for example `@honeycombio/opentelemetry-web` with `GlobalErrorsInstrumentation`) work unchanged.

What you get by swapping:

- **A cache bounded by disk size, not entry count.** Honeycomb's processor holds up to `source_map_cache_size` parsed source maps in RAM (default 128) and re-parses on every eviction and restart. This processor compiles each map and bundle to Traceway's `.tw` binary format once, then memory-maps it from a local cache directory bounded by `cache_max_mb` or `cache_max_disk_pct` (a percentage of the filesystem the cache lives on). Resident memory tracks the hot set; restarts warm from disk; corpus size is a disk budget, not a RAM budget.
- **Pure Go, no cgo.** Honeycomb's processor links Sentry's `symbolic` C library and requires a glibc base image. This one is pure Go by default and runs in any image, including scratch.
- **Function names from scope analysis.** When the minified bundle is in the store (it has to be, for `sourceMappingURL` discovery), enclosing function names are resolved through bundle scope analysis, the same approach as Sentry's symbolic.

## Usage

The processor ships as a public package of the `github.com/tracewayapp/traceway/backend` module; the entry point is `otelprocessor.NewFactory()`. Add it to an [OpenTelemetry Collector Builder](https://opentelemetry.io/docs/collector/custom-collector/) manifest (the `import` line points the builder at the package inside the module):

```yaml
processors:
  - gomod: github.com/tracewayapp/traceway/backend v1.8.0
    import: github.com/tracewayapp/traceway/backend/app/symbolicator/otelprocessor
```

The processor first ships in `v1.8.0`; use that tag or any newer one.

Or, when assembling a collector programmatically, register the factory directly:

```go
import "github.com/tracewayapp/traceway/backend/app/symbolicator/otelprocessor"

factory := otelprocessor.NewFactory()
factories.Processors[factory.Type()] = factory
```

Then reference it in your collector configuration:

```yaml
processors:
  source_map_symbolicator:
    source_map_store: file_store
    local_source_maps:
      path: /sourcemaps
    cache_dir: /var/cache/symbolicator
    cache_max_disk_pct: 50

service:
  pipelines:
    traces:
      processors: [source_map_symbolicator]
    logs:
      processors: [source_map_symbolicator]
```

## How it works

For each span, span event, or log record carrying `exception.stacktrace`:

1. Frames come from the structured parallel arrays (`exception.structured_stacktrace.{urls,functions,lines,columns}`) when present, or from parsing the raw stack string (V8 and Firefox formats, including `eval`, `async`, and `[as alias]` frames).
2. Each frame's URL basename (optionally prefixed by the resource's `app.debug.source_map_uuid`) is fetched from the store, its `//# sourceMappingURL=` comment is followed to the map (inline `data:` URIs supported), and both are compiled into a resolver.
3. The frame resolves to the original file, line, column, and enclosing function name. `exception.stacktrace` is rewritten, the structured arrays are rewritten in place, and the originals are preserved under `.original` keys.
4. `exception.symbolicator.failed`, `exception.symbolicator.error`, and `exception.symbolicator.parsing_method` report the outcome per record.

## Configuration

| Key | Default | Description |
|-----|---------|-------------|
| `source_map_store` | `file_store` | `file_store`, `s3_store`, or `gcs_store` |
| `local_source_maps.path` | `.` | Root directory for `file_store` |
| `s3_source_maps.region` / `.bucket` / `.prefix` | | S3 location; credentials from the default AWS chain |
| `gcs_source_maps.bucket` / `.prefix` | | GCS location; credentials from ADC |
| `timeout` | `5s` | Per-fetch budget for store reads |
| `cache_dir` | `""` | Directory for the `.tw` disk cache; empty disables the disk tier |
| `cache_max_mb` | `2048` | Byte cap for the disk cache, LRU-evicted |
| `cache_max_disk_pct` | `0` | Cap as a percentage of the cache directory's filesystem; when both caps are set, the smaller wins |
| `source_map_cache_size` | `128` | Max open resolvers held in memory (cheap mmap handles when the disk tier is on) |
| `preserve_stack_trace` | `true` | Keep originals under `exception.stacktrace.original` and the `.original` array keys |
| `build_uuid_attribute_key` | `app.debug.source_map_uuid` | Resource attribute used as a store key prefix |
| `language_attribute_key` | `telemetry.sdk.language` | Attribute checked against `allowed_languages` |
| `allowed_languages` | `[]` | When set, only records with a matching language are processed |

All attribute key names (`stack_trace_attribute_key`, `urls_attribute_key`, `symbolicator_failure_attribute_key`, and the rest) are remappable with the same configuration keys and defaults as Honeycomb's processor.

A failed fetch is negative-cached for one minute per bundle URL, so a missing upload cannot turn an error storm into a store-request storm.

## Store layout

The store holds your build output as-is: minified bundles next to their maps, addressed by basename. Re-deploys with content-hashed filenames coexist; stable filenames are overwritten by the newest upload. With `app.debug.source_map_uuid` set on the client's resource, artifacts live under that uuid as a directory prefix, isolating builds completely.

## Links

- [Traceway](https://tracewayapp.com), the MIT-licensed error tracking platform this engine comes from
- [Hosted docs for this processor](https://docs.tracewayapp.com/learn/collector-symbolicator), including the builder walkthrough
- [Symbolication pipeline internals](https://docs.tracewayapp.com/learn/symbolication-js), including the `.tw` format
- [Honeycomb's processor](https://github.com/honeycombio/opentelemetry-collector-symbolicator), whose configuration surface this matches
