---
name: isutools-prometheus
description: Query and analyze Prometheus metrics exposed by isucon-go-tools (isutools). Use when the user wants to inspect ISUCON performance data — slow endpoints, slow SQL queries, cache hit rate, DB connection pool stats, lock contention, queue depth, object pool usage, or benchmark scores — emitted by an application instrumented with the isutools library. Triggers on phrases like "isutools metrics", "isucon-go-tools metrics", "isutools_api_*", "isutools_db_*", "ISUCON のメトリクス", "/metrics endpoint of isutools", or when asked to query the Prometheus endpoint of an isutools-enabled app.
---

# isutools Prometheus Metrics

Query the Prometheus metrics emitted by [isucon-go-tools](https://github.com/mazrean/isucon-go-tools) v2 to analyze the runtime behavior of an ISUCON web application.

## Endpoint

isutools exposes Prometheus metrics on a dedicated HTTP server inside the application process.

| Setting          | Default              | Env var          |
|------------------|----------------------|------------------|
| Listen address   | `:6060`              | `ISUTOOLS_ADDR`  |
| Metrics path     | `/metrics`           | (fixed)          |
| Enable flag      | `true`               | `ISUTOOLS_ENABLE`|
| Hostname label   | (empty)              | `ISUTOOLS_HOST_NAME` |

Quick check:

```bash
curl -s http://localhost:6060/metrics | grep '^isutools_'
```

All metrics use the `isutools` namespace. Subsystems: `api`, `db`, `cache`, `locker`, `pool`, `queue`, `benchmark`.

If the user is running Prometheus / Grafana, query through their `http_api_v1` (`/api/v1/query`, `/api/v1/query_range`). Otherwise scrape `/metrics` directly with `curl` and grep for the metric name.

## How to query

1. **Direct scrape** (no Prometheus server needed): `curl -s http://<host>:6060/metrics | grep '^isutools_<subsystem>_<name>'`
2. **PromQL** (Prometheus/Grafana): use the metric name and labels listed below. PromQL examples are given for each common analysis.
3. **Filter by labels**: every PromQL example below can be narrowed with `{label="value"}`. Be aware of high-cardinality labels (`url`, `query`) — avoid wide regex matches on a busy app.

## Metric reference

### `api` subsystem — HTTP server (`http/prometheus.go`)

Instrumented for net/http, Echo, Gin, Fiber, and FastHTTP via the wrappers in `http/`.

| Metric | Type | Labels | Notes |
|---|---|---|---|
| `isutools_api_request_total` | Counter | `code`, `method`, `host`, `url` | Total HTTP requests. |
| `isutools_api_request_duration_seconds` | Histogram | `code`, `method`, `url` | `prometheus.DefBuckets`. |
| `isutools_api_request_size_bytes` | Histogram | `code`, `method`, `url` | Buckets 1KB…10MB. |
| `isutools_api_response_size_bytes` | Histogram | `code`, `method`, `url` | Buckets 1KB…10MB. |
| `isutools_api_flow_total` | Counter | `source_method`, `source_path`, `target_method`, `target_path` | Endpoint-to-endpoint transitions tracked via cookie. |

`url` is normalized by `FilterFunc` in `http/prometheus.go`: UUIDs become `<uuid>`, digit runs become `<number>`. Use the normalized form when filtering.

### `db` subsystem — `database/sql` wrapper (`db/prometheus.go`, `db/sql.go`)

| Metric | Type | Labels | Notes |
|---|---|---|---|
| `isutools_db_query_count` | Counter | `driver`, `addr`, `query` | `query` is a normalized SQL string. High cardinality. |
| `isutools_db_query_duration_seconds` | Histogram | `driver`, `addr`, `query` | `prometheus.DefBuckets`. |
| `isutools_db_max_open_connections` | Gauge | `driver`, `addr`, `connection_id` | From `sql.DB.Stats().OpenConnections`. |
| `isutools_db_connection_pool` | Gauge | `driver`, `addr`, `connection_id`, `status` (`idle`/`open`/`in_use`) | Three series per pool. |
| `isutools_db_wait_count` | Gauge | `driver`, `addr`, `connection_id` | Cumulative `WaitCount`. |
| `isutools_db_wait_duration` | Gauge | `driver`, `addr`, `connection_id` | Cumulative wait, **nanoseconds**. |
| `isutools_db_max_idle_closed` | Gauge | `driver`, `addr`, `connection_id` | |
| `isutools_db_max_lifetime_closed` | Gauge | `driver`, `addr`, `connection_id` | |
| `isutools_db_max_idle_time_closed` | Gauge | `driver`, `addr`, `connection_id` | |

### `cache` subsystem — `motoki317/sc` and isutools maps/slices (`cache/sc.go`)

| Metric | Type | Labels | Source |
|---|---|---|---|
| `isutools_cache_hit_count` | Gauge | `name`, `stat` (`hit`/`grace_hit`/`miss`/`replace`) | `sc.Cache` stats. |
| `isutools_cache_load_count` | Gauge | `name`, `status` (`hit`/`miss`) | `Map`/`AtomicMap`. |
| `isutools_cache_store_count` | Gauge | `name`, `status` (`replace`/`new`/`remove`) | `Map`/`AtomicMap`. |
| `isutools_cache_index_access` | Histogram | `name` | `Slice` index distribution (linear buckets 0…size). |
| `isutools_cache_length` | Gauge | `name` | Current length of `Slice` / `NoDeleteSlice`. |

### `locker` subsystem (`locker/locker.go`)

| Metric | Type | Labels | Notes |
|---|---|---|---|
| `isutools_locker_index_access` | Histogram | `name`, `type` (`read`/`write`) | Lock acquisition latency (seconds). |

### `pool` subsystem (`pool/pool.go`)

| Metric | Type | Labels | Notes |
|---|---|---|---|
| `isutools_pool_count` | Counter | `name`, `type` (`alloc`/`get`/`put`) | Generic `Pool`, `SlicePool`, `MapPool`. |

### `queue` subsystem (`queue/channel.go`)

| Metric | Type | Labels | Notes |
|---|---|---|---|
| `isutools_queue_counter` | Gauge | `name`, `status` (`in`/`out`) | Use `in - out` for current depth. |

### `benchmark` subsystem (`internal/benchmark/status.go`)

| Metric | Type | Labels | Notes |
|---|---|---|---|
| `isutools_benchmark_score` | Gauge | — | Last benchmark score. Updated via `POST /benchmark/score`. |
| `isutools_benchmark_duration` | Gauge | — | Last benchmark duration (seconds). |

## PromQL recipes for ISUCON analysis

### Top-N slowest endpoints (p95 over last 1m)

```promql
topk(10,
  histogram_quantile(0.95,
    sum by (method, url, le) (
      rate(isutools_api_request_duration_seconds_bucket[1m])
    )
  )
)
```

### Endpoints contributing most total time (the score-killers)

```promql
topk(10,
  sum by (method, url) (
    rate(isutools_api_request_duration_seconds_sum[1m])
  )
)
```

### Request rate and error rate

```promql
sum by (method, url) (rate(isutools_api_request_total[1m]))

sum by (url) (rate(isutools_api_request_total{code=~"5.."}[1m]))
  / ignoring(code) sum by (url) (rate(isutools_api_request_total[1m]))
```

### Slowest SQL queries (p95)

```promql
topk(10,
  histogram_quantile(0.95,
    sum by (query, le) (
      rate(isutools_db_query_duration_seconds_bucket[1m])
    )
  )
)
```

### SQL queries by total time consumed

```promql
topk(10,
  sum by (query) (
    rate(isutools_db_query_duration_seconds_sum[1m])
  )
)
```

### DB connection pool saturation

```promql
isutools_db_connection_pool{status="in_use"}
  / on(connection_id) isutools_db_max_open_connections
```

If this approaches 1.0, the pool is the bottleneck. Also check `isutools_db_wait_count` increasing and `isutools_db_wait_duration` (nanoseconds) growing.

### Cache hit rate (sc.Cache)

```promql
sum by (name) (isutools_cache_hit_count{stat=~"hit|grace_hit"})
  / sum by (name) (isutools_cache_hit_count{stat=~"hit|grace_hit|miss"})
```

### Map cache hit rate

```promql
sum by (name) (isutools_cache_load_count{status="hit"})
  / sum by (name) (isutools_cache_load_count)
```

### Lock contention (RWMutex acquisition latency)

```promql
histogram_quantile(0.99,
  sum by (name, type, le) (
    rate(isutools_locker_index_access_bucket[1m])
  )
)
```

### Object pool effectiveness

```promql
sum by (name) (rate(isutools_pool_count{type="get"}[1m]))
  / sum by (name) (rate(isutools_pool_count{type="alloc"}[1m]))
```

A high ratio means most `get`s reuse a pooled object instead of allocating.

### Queue depth

```promql
sum by (name) (isutools_queue_counter{status="in"})
  - sum by (name) (isutools_queue_counter{status="out"})
```

### Benchmark score over time

```promql
isutools_benchmark_score
isutools_benchmark_duration
```

## Quick `curl`-only recipes (no Prometheus server)

When only `/metrics` is reachable, parse with grep/awk. Examples:

```bash
# All HTTP request counters, sorted by count desc
curl -s http://localhost:6060/metrics \
  | awk '/^isutools_api_request_total\{/ {print $NF, $0}' \
  | sort -rn | head

# Top SQL queries by count
curl -s http://localhost:6060/metrics \
  | awk '/^isutools_db_query_count\{/ {print $NF, $0}' \
  | sort -rn | head

# DB connection pool snapshot
curl -s http://localhost:6060/metrics | grep -E '^isutools_db_(connection_pool|max_open_connections|wait_)'

# Cache hit/miss snapshot
curl -s http://localhost:6060/metrics | grep '^isutools_cache_'
```

## Workflow when the user asks about isutools metrics

1. Confirm the endpoint is reachable (`curl -sf http://<host>:6060/metrics -o /dev/null && echo ok`). If not, check `ISUTOOLS_ENABLE`, `ISUTOOLS_ADDR`, and that the app actually imports an isutools subpackage.
2. Identify the question category — latency, throughput, errors, DB, cache, lock, pool, queue, benchmark — and pick the matching metric(s) above.
3. Use PromQL when a Prometheus server is available; otherwise fall back to `curl | grep`.
4. Watch for high-cardinality labels (`url`, `query`): aggregate first with `sum by (...)` before applying `histogram_quantile`.
5. Report findings with the exact metric name and labels used so the user can reproduce the query.

## Gotchas

- Metric `Help` strings are mostly empty; rely on this document, not `# HELP` lines.
- `isutools_cache_hit_count` is a **Gauge**, not a Counter — it reports cumulative `sc.Stats()` snapshots, so use it directly (no `rate()`).
- `isutools_db_wait_duration` is in **nanoseconds**.
- `url` and `query` labels are pre-normalized (UUIDs → `<uuid>`, digits → `<number>` for URLs; SQL is normalized by the wrapper). Use the normalized form when filtering.
- The metrics server runs on a separate port (`:6060` by default), not on the application's main port.
- If `ISUTOOLS_ENABLE=false`, none of the instrumentation runs and `/metrics` will not include `isutools_*` series.
