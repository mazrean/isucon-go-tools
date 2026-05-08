---
name: isutools-prometheus
description: Query isutools (isucon-go-tools) metrics via PromQL on a Prometheus server. Use when the user wants to analyze ISUCON performance — slow endpoints, slow SQL queries, cache hit rate, DB connection pool, lock contention, queue depth, object pool usage, or benchmark scores — using Prometheus (`/api/v1/query`, `/api/v1/query_range`, Grafana, `promtool`). Triggers on phrases like "isutools metrics", "isucon-go-tools metrics", "isutools_api_*", "isutools_db_*", "ISUCON のメトリクスをクエリ", "PromQL for isutools".
---

# Querying isutools Metrics via Prometheus

PromQL reference for metrics emitted by [isucon-go-tools](https://github.com/mazrean/isucon-go-tools) v2. Assume a Prometheus server is already scraping the application — this skill only covers **querying**, not setting up the exporter.

All metrics use the `isutools` namespace. Subsystems: `api`, `db`, `cache`, `locker`, `pool`, `queue`, `benchmark`.

## How to issue queries

- **Prometheus HTTP API**: `GET /api/v1/query?query=<PromQL>` (instant), `GET /api/v1/query_range?query=...&start=...&end=...&step=...` (range).
- **`promtool`**: `promtool query instant http://<prom>:9090 '<PromQL>'` / `promtool query range ...`.
- **Grafana**: paste the PromQL into a panel.

When invoking via `curl`, URL-encode the PromQL expression. Example:

```bash
curl -sG http://prometheus:9090/api/v1/query \
  --data-urlencode 'query=topk(5, sum by (url) (rate(isutools_api_request_duration_seconds_sum[1m])))'
```

## Metric reference

### `api` — HTTP server

| Metric | Type | Labels |
|---|---|---|
| `isutools_api_request_total` | Counter | `code`, `method`, `host`, `url` |
| `isutools_api_request_duration_seconds` | Histogram | `code`, `method`, `url` |
| `isutools_api_request_size_bytes` | Histogram | `code`, `method`, `url` |
| `isutools_api_response_size_bytes` | Histogram | `code`, `method`, `url` |
| `isutools_api_flow_total` | Counter | `source_method`, `source_path`, `target_method`, `target_path` |

`url` is pre-normalized: UUIDs → `<uuid>`, digit runs → `<number>`. Use the normalized form when filtering.

### `db` — `database/sql` wrapper

| Metric | Type | Labels |
|---|---|---|
| `isutools_db_query_count` | Counter | `driver`, `addr`, `query` |
| `isutools_db_query_duration_seconds` | Histogram | `driver`, `addr`, `query` |
| `isutools_db_max_open_connections` | Gauge | `driver`, `addr`, `connection_id` |
| `isutools_db_connection_pool` | Gauge | `driver`, `addr`, `connection_id`, `status` (`idle`/`open`/`in_use`) |
| `isutools_db_wait_count` | Gauge | `driver`, `addr`, `connection_id` |
| `isutools_db_wait_duration` | Gauge | `driver`, `addr`, `connection_id` |
| `isutools_db_max_idle_closed` | Gauge | `driver`, `addr`, `connection_id` |
| `isutools_db_max_lifetime_closed` | Gauge | `driver`, `addr`, `connection_id` |
| `isutools_db_max_idle_time_closed` | Gauge | `driver`, `addr`, `connection_id` |

`query` is a normalized SQL string (high cardinality). `isutools_db_wait_duration` is in **nanoseconds**.

### `cache` — `motoki317/sc` and isutools maps/slices

| Metric | Type | Labels |
|---|---|---|
| `isutools_cache_hit_count` | Gauge | `name`, `stat` (`hit`/`grace_hit`/`miss`/`replace`) |
| `isutools_cache_load_count` | Gauge | `name`, `status` (`hit`/`miss`) |
| `isutools_cache_store_count` | Gauge | `name`, `status` (`replace`/`new`/`remove`) |
| `isutools_cache_index_access` | Histogram | `name` |
| `isutools_cache_length` | Gauge | `name` |

`isutools_cache_hit_count` is a **Gauge** that mirrors the cumulative `sc.Stats()` snapshot — query it directly, don't wrap it in `rate()`.

### `locker` — RWMutex

| Metric | Type | Labels |
|---|---|---|
| `isutools_locker_index_access` | Histogram | `name`, `type` (`read`/`write`) |

### `pool` — object pool

| Metric | Type | Labels |
|---|---|---|
| `isutools_pool_count` | Counter | `name`, `type` (`alloc`/`get`/`put`) |

### `queue` — channel-backed queue

| Metric | Type | Labels |
|---|---|---|
| `isutools_queue_counter` | Gauge | `name`, `status` (`in`/`out`) |

### `benchmark`

| Metric | Type | Labels |
|---|---|---|
| `isutools_benchmark_score` | Gauge | — |
| `isutools_benchmark_duration` | Gauge | — |

## PromQL recipes

### Slowest endpoints (p95 over the last 1m)

```promql
topk(10,
  histogram_quantile(0.95,
    sum by (method, url, le) (
      rate(isutools_api_request_duration_seconds_bucket[1m])
    )
  )
)
```

### Endpoints consuming the most total time (the score-killers)

```promql
topk(10,
  sum by (method, url) (
    rate(isutools_api_request_duration_seconds_sum[1m])
  )
)
```

### Throughput and 5xx error rate per endpoint

```promql
sum by (method, url) (rate(isutools_api_request_total[1m]))

sum by (url) (rate(isutools_api_request_total{code=~"5.."}[1m]))
  / ignoring(code) sum by (url) (rate(isutools_api_request_total[1m]))
```

### Endpoint transition flows

```promql
topk(20,
  sum by (source_path, target_path) (
    rate(isutools_api_flow_total[5m])
  )
)
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

Approaching 1.0 means the pool is the bottleneck. Cross-check with growth in `isutools_db_wait_count` and `isutools_db_wait_duration`:

```promql
rate(isutools_db_wait_count[1m])
rate(isutools_db_wait_duration[1m]) / 1e9    # seconds per second of wait
```

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

### Slice index access distribution (p99)

```promql
histogram_quantile(0.99,
  sum by (name, le) (
    rate(isutools_cache_index_access_bucket[1m])
  )
)
```

### Lock contention (RWMutex acquisition latency p99)

```promql
histogram_quantile(0.99,
  sum by (name, type, le) (
    rate(isutools_locker_index_access_bucket[1m])
  )
)
```

### Object pool reuse ratio

```promql
sum by (name) (rate(isutools_pool_count{type="get"}[1m]))
  / sum by (name) (rate(isutools_pool_count{type="alloc"}[1m]))
```

A high ratio means most `get`s reuse a pooled object instead of allocating.

### Queue depth and throughput

```promql
sum by (name) (isutools_queue_counter{status="in"})
  - sum by (name) (isutools_queue_counter{status="out"})

sum by (name) (rate(isutools_queue_counter{status="in"}[1m]))
sum by (name) (rate(isutools_queue_counter{status="out"}[1m]))
```

### Benchmark score and duration

```promql
isutools_benchmark_score
isutools_benchmark_duration
```

## Workflow when answering an isutools metrics question

1. Map the question to a subsystem and metric from the reference above.
2. Aggregate first with `sum by (...)` before applying `histogram_quantile` — `url` and `query` are high-cardinality, and querying buckets without aggregation is both slow and wrong.
3. Pick the time window deliberately: `[1m]` for live load, `[5m]`–`[15m]` for trends, range queries for benchmarks.
4. Report the exact PromQL used so the user can paste it into Grafana / `promtool` and verify.

## Gotchas

- Histograms expose `_bucket` / `_sum` / `_count` — always `rate(..._bucket[...])` and `sum by (..., le)` before `histogram_quantile`.
- `isutools_cache_hit_count` is a Gauge (cumulative snapshot). Use `delta(...[5m])` if you need the change over a window; never `rate()`.
- `isutools_db_wait_duration` is in **nanoseconds** — divide by `1e9` for seconds.
- Counter resets happen on app restart; use `rate()` / `increase()` (which handle resets) rather than raw `... - ... offset ...`.
- `url` and `query` labels are pre-normalized inside the application; queries should use the normalized form (e.g. `/users/<number>`, not `/users/42`).
- `isutools_api_request_total` carries a `host` label that the duration/size histograms do not; don't `on()`-join across them on `host`.
