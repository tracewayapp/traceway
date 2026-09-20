"""Print an isolated clickhouse-local benchmark; run against a fresh --path."""
import hashlib
from pathlib import Path
from uuid import UUID

ROOT = Path(__file__).resolve().parents[2]
queries = []

def varint(n):
    data = bytearray()
    while n > 127:
        data.append((n & 127) | 128)
        n >>= 7
    return bytes(data + bytes([n]))

def field(number, data):
    return varint((number << 3) | 2) + varint(len(data)) + data

sql_text = b'select users where user_id=123 ' * 8
attribute = field(1, b'db.statement') + field(2, field(1, sql_text))
# Additional preserved event detail is deliberately repetitive; actual payloads vary.
event = field(2, b'database event') + field(3, field(1, b'detail') + field(2, field(1, b'event context ' * 64)))
span_tail = field(5, b'db.query') + field(9, attribute) + field(11, event)

hot_columns = 'project_id,otel_trace_id,span_id,parent_span_id,distributed_trace_id,name,start_time,duration,attributes,span_kind,status_code,service_name,scope_name,start_time_unix_nano'


def query(label, sql):
    queries.append(f"SELECT '{label}' AS benchmark;\n{sql.rstrip(';')} ;")

query('source', '''CREATE TABLE source ENGINE=MergeTree ORDER BY number AS
 SELECT number,
 UUIDNumToString(MD5(toString(intDiv(number,10000)))) AS project_id,
 lower(hex(MD5(toString(intDiv(number,10))))) AS otel_trace_id,
 lower(substring(hex(MD5(toString(number))),1,16)) AS span_id,
 if(number%10=0,'',lower(substring(hex(MD5(toString(number-1))),1,16))) AS parent_span_id,
 now64(9) AS start_time,
 toJSONString(map('db.statement',repeat('select users where user_id=123 ',8))) AS attributes
 FROM numbers(1000000)''')
query('legacy_ddl', '''CREATE TABLE legacy_spans (
 id UUID, trace_id UUID, project_id UUID, name String, start_time DateTime64(6),duration Int64,recorded_at DateTime,
 parent_span_id Nullable(UUID),attributes String,
 INDEX idx_trace_id trace_id TYPE bloom_filter(0.001) GRANULARITY 1
) ENGINE=MergeTree PARTITION BY toYYYYMMDD(recorded_at) ORDER BY (project_id,trace_id,start_time)''')
query('spans_ddl', "CREATE TABLE spans AS legacy_spans")
query('v2_columns', (ROOT / 'backend/app/migrations/ch/0085_complete_spans.up.sql').read_text().strip())
query('legacy_insert', '''INSERT INTO legacy_spans SELECT
 toUUID(UUIDNumToString(toFixedString(unhex(concat('0000000000000000',span_id)),16))),
 toUUID(UUIDNumToString(toFixedString(unhex(otel_trace_id),16))),toUUID(project_id),'db.query',start_time,1000000,start_time,
 toUUID(UUIDNumToString(toFixedString(unhex(concat('0000000000000000',parent_span_id)),16))), attributes
 FROM source WHERE number%10 != 0''')
scope_pb = field(1, field(1, b'test'))
resource_pb = field(1, field(1, field(1, b'service.name') + field(2, field(1, b'api'))))
query('v2_insert', f'''INSERT INTO spans (
 id,trace_id,project_id,name,start_time,duration,recorded_at,parent_span_id,attributes,
 schema_version,otel_trace_id,span_id,distributed_trace_id,span_kind,status_code,service_name,scope_name,
 start_time_unix_nano,end_time_unix_nano,span_attributes,resource,scope,events,resource_pb,scope_pb,span_pb)
 SELECT
 toUUID(UUIDNumToString(MD5(concat(project_id,otel_trace_id,span_id)))),
 toUUID(UUIDNumToString(toFixedString(unhex(otel_trace_id),16))),toUUID(project_id),'db.query',start_time,1000000,start_time,
 if(parent_span_id='',NULL,toUUID(UUIDNumToString(toFixedString(unhex(concat('0000000000000000',parent_span_id)),16)))),'{{}}',
 2,otel_trace_id,span_id,toUUID(UUIDNumToString(toFixedString(unhex(otel_trace_id),16))),1,0,'api','test',
 toUnixTimestamp64Nano(start_time),toUnixTimestamp64Nano(start_time)+1000000,
 CAST(JSONExtract(attributes,'Map(String, String)') AS Map(LowCardinality(String), String)),
 '{{"attributes":{{"service.name":"api"}}}}', '{{"name":"test","attributes":{{}}}}',
 concat('[{{"time_unix_nano":',toString(toUnixTimestamp64Nano(start_time)+1),',"name":"database event","attributes":{{"detail":"event context"}},"dropped_attributes_count":0}}]'),
 unhex('{resource_pb.hex()}'),unhex('{scope_pb.hex()}'),
 concat(unhex('0a10'),unhex(otel_trace_id),unhex('1208'),unhex(span_id),unhex('{span_tail.hex()}'))
 FROM source''')
for count in (1, 100):
    numbers = [(i*9973+12340)//10*10 for i in range(count)]
    for table, trace_col in [('legacy_spans','trace_id'),('spans','trace_id')]:
        conditions=[]
        pairs=[]
        for n in numbers:
            project = UUID(hashlib.md5(str(n//10000).encode()).hexdigest())
            trace = UUID(hashlib.md5(str(n//10).encode()).hexdigest())
            pairs.append(f"(toUUID('{project}'),toUUID('{trace}'))")
            conditions.append(f"(project_id=toUUID('{project}') AND {trace_col}=toUUID('{trace}'))")
        where = 'schema_version=2 AND (project_id,trace_id) IN (' + ','.join(pairs) + ')' if table == 'spans' else ' OR '.join(conditions)
        columns = hot_columns.replace("attributes,", "toJSONString(span_attributes) AS attributes,") if table == "spans" else "*"
        sql = f"SELECT {columns} FROM {table} WHERE " + where + ' LIMIT 20001'
        query(f'{table}_{count}_plan', 'EXPLAIN indexes=1 ' + sql)
        query(f'{table}_{count}_read', f'SELECT count(),sum(cityHash64(toJSONString(tuple(*)))) FROM ({sql}) FORMAT JSON')
query('attribute_path_plan', "EXPLAIN indexes=1 SELECT count() FROM spans WHERE schema_version=2 AND span_attributes['db.statement'] LIKE 'select%' ")
query('attribute_path_read', "SELECT count() FROM spans WHERE schema_version=2 AND span_attributes['db.statement'] LIKE 'select%' FORMAT JSON")
query('resource_like_plan', '''EXPLAIN indexes=1 SELECT count() FROM spans WHERE schema_version=2 AND resource LIKE '%"service.name":"api"%' ''')
query('resource_like_read', '''SELECT count() FROM spans WHERE schema_version=2 AND resource LIKE '%"service.name":"api"%' FORMAT JSON''')
query('payload_read', "SELECT count(), sum(length(resource_pb) + length(scope_pb) + length(span_pb)) FROM spans WHERE schema_version=2 FORMAT JSON")
query('storage', "SELECT table, sum(rows) AS rows, sum(bytes_on_disk) AS bytes, sum(data_compressed_bytes) AS compressed, sum(marks_bytes) AS marks FROM system.parts WHERE active AND table IN ('spans','legacy_spans') GROUP BY table FORMAT JSON")
print('\n\n'.join(queries))
