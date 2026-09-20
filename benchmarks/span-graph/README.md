# Single-table span storage benchmarks

> These measurements were taken on the layout that came before the V2 tables, where the new columns were added to the old `spans` table by a migration numbered 0085. That migration never shipped. `spans_v2` (now created by `ch/0085_create_spans_v2`) carries the same column types and the same `Map` of attributes, so everything below about attribute storage, the Map against `JSON`, and the fixed fields as columns still applies. The comparison against the legacy writer is history: that writer no longer exists.

Measured September 18, 2026 against `clickhouse/clickhouse-server:25.8-alpine` (25.8.33) in local arm64 Docker. The machine was under unrelated load, so every Go figure is the minimum of three rounds. Read the numbers as ratios between layouts on one machine. They are not production capacity figures.

There are three sources of evidence here. Each has a command below that reproduces it.

1. **Real writers.** The production `InsertAsync` paths for legacy and complete spans, against a table built from the real migration.
2. **Table shape comparison.** The same spans written into seven candidate layouts. This is the evidence for the layout in migration 0085.
3. **SQL probe.** A `clickhouse-local` script that applies migration 0085 to a scratch table and measures storage, plans and reads at one million rows.

## The layout in migration 0085

Complete OTel spans live in the existing `spans` table. Nothing is stored anywhere else.

- Every scalar field is a plain column.
- `span_attributes` is a `Map(LowCardinality(String), String)`, filled the way the OpenTelemetry Collector's ClickHouse exporter does it: keys verbatim, scalars as text, arrays and key/value lists as JSON. The first version of this layout used native `JSON` here. "Attributes as a Map" below explains the change.
- `resource` and `scope` are JSON text in `LowCardinality(String)` columns. The schema URLs and scope version are `LowCardinality` too.
- `events` and `links` are JSON text.
- The lossless payload is three protobuf columns: `resource_pb` and `scope_pb` as `LowCardinality(String)`, and `span_pb` per span. `AssembleOtelPayload` joins them back into the original `ResourceSpans`, including unknown protobuf fields at all three levels.

The reason for this shape is the second table below. The cost of the earlier draft was not the number of columns, and not JSON as such. It was writing the same resource again for every span, once as native JSON and once inside a per-span protobuf blob. A resource is identical for every span a process exports, so a dictionary column stores it once per block instead of once per row, on the wire and on disk.

## 1. Real writers

60,000 spans, 12 per trace, six services, a 29-attribute resource, 2 to 11 span attributes, events on a fifth of spans. Merges stopped, table truncated between rounds.

| | Legacy V1 | Complete V2 | Ratio |
| --- | ---: | ---: | ---: |
| Insert, 500 spans per batch | 12.1 to 15.2 µs/span | 28.3 to 31.3 µs/span (32,000 to 35,400 spans/s) | 1.9x to 2.6x |
| Insert, 6,000 spans per batch | 4.4 to 4.7 µs/span | 13.5 to 15.8 µs/span (63,400 to 74,000 spans/s) | 3.1x to 3.4x |
| Go-side row preparation | | 5.2 µs/span | |
| Disk, unmerged parts | | 118 bytes/span | |

The ranges are two separate runs on a machine under unrelated load. 500 spans per batch is the realistic case, because an OTLP exporter sends about that many per request and each request is one insert.

Before this layout and the shared row builder, the same comparison measured about 207 µs per span for V2 at large batches against 5.8 for legacy, roughly 36 times slower, with 141 µs of it spent in Go. That code is gone, so the figure cannot be reproduced from this tree. The table shape comparison below still reproduces the storage half of it.

`BenchmarkOtelSpanRowScalars` and `BenchmarkOtelSpanRowTextStorage` in `backend/app/repositories/telemetry/shared` cover the Go side without a database. The text path, which SQLite and DuckDB use, measured 16 µs per span when it marshalled every span to protobuf JSON and parsed it back to split it into columns. Since September 19 all three backends share one encoder (`shared/otel_json.go`) and store the same content in `span_attributes`, `resource`, `scope`, `events` and `links`. The text path now measures 5.3 µs per span, with 31,421 allocations per 512-span batch instead of 83,001.

## 2. Table shape comparison

120,000 of the same spans into each layout. Every layout except the first two keeps the full lossless payload, so none of them loses data. Insert columns are driver plus server time with rows prepared in advance.

| Layout | Insert at 6,000 | Insert at 500 | Disk | Column streams |
| --- | ---: | ---: | ---: | ---: |
| `legacy_shape`: 8 scalars and one JSON string | 2.9 µs | 10.2 µs | 48 B | 11 |
| `scalars_only`: the 28 V2 scalar columns | 2.8 µs | 10.1 µs | 69 B | 42 |
| `draft_five_native_json`: the superseded 0085 draft | 34.8 µs | 41.2 µs | 139 B | 283 |
| `one_native_json`: one native JSON column and the full blob | 37.9 µs | 45.7 µs | 139 B | 311 |
| `one_string_json`: one JSON string and the full blob | 27.4 µs | 38.3 µs | 143 B | 44 |
| `adopted_text_attributes`: adopted, attributes as text | 8.7 µs | 18.9 µs | 117 B | 54 |
| `native_json_attributes`: the first version of 0085, measured as `adopted` on September 18 | 8.7 µs | 21.5 µs | 117 B | 133 |

What this shows:

- Twenty more scalar columns cost nothing. `scalars_only` matches `legacy_shape`.
- Folding everything into one native JSON column is the worst option on every measure.
- One JSON string is cheaper than native JSON but still repeats the resource on every row.
- The adopted layout inserts about four times faster than the draft at large batches and about twice as fast at 500, is 16% smaller, and has fewer than half the column streams.
- With a single writer and merges stopped, native `span_attributes` costs 2.6 µs per span at small batches and nothing at large ones. With concurrent writers and merges running it costs far more, which is what decided against it.

Query latency on the same 120,000-row tables:

| Query | Draft | Adopted |
| --- | ---: | ---: |
| Span attribute filter on `http.route`, native subcolumn | 10.9 ms | 5.5 ms |
| Same filter with attributes as text and `JSONExtractString` | | 27.5 ms |
| Resource filter on `k8s.pod.name` | 8.1 ms | 6.6 ms with `LIKE` |
| Same resource filter with `JSONExtractString` | | 133.5 ms |

That was the case for native `span_attributes`. It lost to write capacity, see "Attributes as a Map". The numbers still set one rule for resource queries, which did not change. ClickHouse evaluates `LIKE` on a `LowCardinality` column once per distinct value, but it evaluates `JSONExtractString` once per row. Filter resources with a `LIKE` on the encoded pair, for example `resource LIKE '%"k8s.pod.name":"cart-7d9f8b6c5-00001"%'`. The text is written by `encoding/json` with sorted keys and no whitespace, so the pattern is stable, and `TestOtelResourceTextIsDeterministic` holds it there. Build the pattern with the same encoder, because it escapes `<`, `>` and `&`, and escape the `LIKE` wildcards `%` and `_` in values. A resource key that becomes hot should get its own `LowCardinality` column, as `service_name` already has.

Many distinct resources degrade this gracefully. `TEST_CLICKHOUSE_BENCH_RESOURCES=5000` gives every trace its own resource, so a resource is shared by only 12 spans. That is adversarial, since a real OTLP request carries one resource for hundreds of spans. The real writers then measured 31.4 µs per span at 500 spans per batch (2.3x legacy), 22.8 µs at 6,000 (5.3x), 12.2 µs of Go preparation and 130 bytes per span. There is no cliff, and it stays well below the draft's 41 µs.

## Attributes as a Map

Measured September 19, 2026 on the same machine. The first version of migration 0085 stored `span_attributes` as native `JSON`. That decision rested on the single-writer figures above. With concurrent writers and merges running the picture changes, because the server parses JSON per row and maintains a subcolumn per key (133 column streams against 57), and merges pay for that again.

| Layout | 8 writers, 500 per insert, merges running | ClickHouse CPU per row | Disk |
| --- | ---: | ---: | ---: |
| `legacy_shape`, not a complete span | 143,500 spans/s | 4.4 µs | 48 B |
| Scalars and the three protobuf columns only, the floor for a lossless span | 67,300 spans/s | 8.8 µs | 107 B |
| **`adopted`: Map attributes, text columns, protobuf** | **56,600 spans/s** | **11.2 µs** | **127 B** |
| `native_json_attributes`, the first version | 43,700 spans/s | 13.5 µs | 117 B |
| OpenTelemetry Collector exporter layout: Map attributes, a resource Map on every row, events and links as arrays | | 11.1 µs, 23.8 µs wall | 110 B |

CPU per row is ClickHouse's own thread CPU from `system.query_log` over 2,160 inserts per layout. It is the figure to trust, because wall time moved by about 2 µs between runs on this machine.

What the Map costs on the read side, on 120,000 rows: a filter on one attribute takes 16.8 ms against 6.6 ms for a native subcolumn and 26.9 ms for `JSONExtractString` on text. Reading one trace with its attributes, which is the only read the product does, is the same (8.8 ms against 9.5 ms). If an attribute search over large ranges is ever built, a native `JSON` column can be added next to the Map in an additive migration.

What it removes: native `JSON` with dotted keys needs ClickHouse 25.8 and the `json_type_escape_dots_in_keys` setting. The Map needs neither. The whole suite passes on ClickHouse 24.8.14 and on 25.8.33, so there is no minimum version to enforce.

### Should the fixed fields be columns or live in the Map

Columns. Measured on a warm server, 1,440 inserts per variant:

| Variant | ClickHouse CPU per row | Disk |
| --- | ---: | ---: |
| Fixed fields as columns, as adopted | 8.0 µs | 126.7 B |
| The nine rarely used fields folded into the Map (`trace_state`, `flags`, `status_message`, the three dropped counts, both schema URLs, `scope_version`) | 7.3 µs | 136.7 B |
| The same nine fields not stored at all | 7.0 µs | 126.6 B |

All nine together cost about 1 µs per row and nothing on disk, because ClickHouse stores mostly default columns sparse (0.00 to 0.02 B per span each). Folding them into the Map saves 0.7 µs, costs 8% more disk, loses their types and mixes synthetic keys with user attributes.

Where the bytes are, per span: `span_pb` 38, `span_id` 17, `id` 16, `span_attributes` 16, `parent_span_id` 10, the four time columns 20, `otel_trace_id` 3, events and links 3, everything else under 1. `id` is never read for complete spans, the reader recomputes it. `otel_trace_id` repeats `trace_id` as text. Dropping `otel_trace_id` and `end_time_unix_nano` measured 9 B less per span with no change in insert cost. They were kept, because they are plain OTel fields and the saving is small.

## 3. SQL probe

One million spans, 100 projects, 100,000 traces, ten spans per trace, two threads and a 1 GiB query memory limit. The legacy baseline keeps nine children per trace. V2 also keeps the roots and the complete source fields. See [recorded results and query plans](lowcardinality-results.json). The table below and that file were recorded on September 18 with the first version of the layout, when `span_attributes` was native `JSON`. The generator now builds the Map layout and runs on ClickHouse 24.8 and 25.8, but these figures have not been re-recorded, so read the attribute rows as history. Storage, the legacy comparison and the resource filter are not affected by that change.

| Measurement | Legacy | V2 |
| --- | ---: | ---: |
| Stored rows | 900,000 | 1,000,000 |
| SQL insert from prepared source | 2.898 s | 5.960 s |
| One trace, graph fields including attributes | 15 ms | 21 ms |
| 100 scattered traces, graph fields including attributes | 329 ms | 259 ms |
| On-disk table size | 22,008,635 bytes | 63,965,890 bytes |
| Granules after primary and Bloom pruning, 100 traces | 87 / 111 | 100 / 190 |
| Native attribute path filter over all V2 rows | | 99 ms |
| Resource `LIKE` filter over all V2 rows | | 12 ms |

This probe cannot show the gain from dictionary-encoding the resource. Its synthetic resource is a single `service.name` attribute, so there is almost nothing to deduplicate, and its inserts are SQL from a prepared source with no Go or driver work. Use it for storage, plans and read behavior at a million rows. Use the Go benchmarks for ingest cost.

## Limitations

- Attributes and the roughly 900-byte protobuf event detail in the SQL probe repeat heavily. Real high-cardinality traffic compresses less well.
- Dictionary encoding works per block. A part that mixes very many distinct resources in one block benefits less. Rows are sorted by project and trace, which keeps a block's resources few, but this should be confirmed on production-like data.
- There was no sustained ingestion, concurrent read load or production-sized retention in any of these runs.
- `clickhouse-local` reports zero rows and bytes read in its JSON statistics. Those counters are unavailable, not evidence of zero work. The recorded `EXPLAIN` output shows the pruning.

## Reproduce

Start a disposable server:

```sh
docker run -d --name span-bench --ulimit nofile=262144:262144 -e CLICKHOUSE_SKIP_USER_SETUP=1 \
  -p 127.0.0.1:19100:9000 clickhouse/clickhouse-server:25.8-alpine
```

The two Go tests behind sections 1 and 2 (`TestOtelSpanInsertThroughput`, `TestOtelSpanSchemaVariants`) were removed together with the legacy span writer they compared against. The row builder benchmark still runs:

```sh
cd backend
go test -run xxx -bench BenchmarkOtelSpanRow -benchtime 2s ./app/repositories/telemetry/shared/
```

SQL probe, in a container without network or mounted data:

```sh
python3 benchmarks/span-graph/generate_clickhouse.py > /tmp/span-graph.sql
docker run --rm --network none --entrypoint clickhouse -i \
  clickhouse/clickhouse-server:25.8-alpine local --path /tmp/span-benchmark \
  --max_threads 2 --max_memory_usage 1073741824 --multiquery --time \
  < /tmp/span-graph.sql > /tmp/span-graph.out 2> /tmp/span-graph.times
```

SQLite and DuckDB have driver-backed query-plan tests using 10,000 spans:

```sh
cd backend
go test ./app/repositories/telemetry -run TestOtelSpanIndexedQueries -count=1 -v
go test -tags telemetry_duckdb ./app/repositories/telemetry -run TestOtelSpanIndexedQueries -count=1 -v
```

## Measure the current span write rate

For ClickHouse Cloud, this query measures completed initial span inserts over a ten-minute window, leaving one minute for query-log flushes. It reports stored rows, not exporter arrivals or peak capacity. Run it against the intended `traceway` database and cluster.

```sql
SELECT
    count() AS completed_insert_queries,
    sum(written_rows) AS span_rows_written,
    round(sum(written_rows) / 600.0, 2) AS span_rows_per_second,
    round(avg(written_rows), 1) AS average_rows_per_insert,
    quantile(0.95)(query_duration_ms) AS insert_p95_ms
FROM clusterAllReplicas('default', system.query_log)
WHERE event_date >= toDate(now() - INTERVAL 11 MINUTE)
  AND event_time >= now() - INTERVAL 11 MINUTE
  AND event_time < now() - INTERVAL 1 MINUTE
  AND type = 'QueryFinish'
  AND is_initial_query = 1
  AND query_kind = 'Insert'
  AND has(tables, 'traceway.spans')
SETTINGS
    max_execution_time = 10,
    max_rows_to_read = 1000000,
    max_bytes_to_read = 268435456,
    max_memory_usage = 268435456,
    read_overflow_mode = 'throw',
    timeout_overflow_mode = 'throw',
    skip_unavailable_shards = 0;
```
