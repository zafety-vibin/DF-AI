# Quickstart: Foundation Infrastructure

**Feature**: 002-foundation-infrastructure

This guide covers using the configuration system, querying monitoring endpoints, and running performance benchmarks.

---

## Configuration Management

### Loading Configuration

The orchestrator loads configuration from `config/orchestrator.yaml` at startup:

```bash
./bin/df-orchestrator.exe --config config/orchestrator.yaml
```

### Configuration File Format

```yaml
# Network settings
listen_port: 5001          # TCP port for DFHack protocol
http_port: 8080            # HTTP port for monitoring API
enable_http_api: true      # Enable HTTP endpoints
read_timeout: 30s
write_timeout: 10s

# Protocol settings
heartbeat_interval: 10s
heartbeat_timeout: 5s
max_message_size: 104857600  # 100 MB

# Reconnection settings
reconnect_delay: 1s
reconnect_max_delay: 30s
reconnect_backoff_factor: 2.0

# Logging
log_level: info       # debug, info, warn, error
log_format: json      # json or text
```

### Hot-Reload Configuration

The orchestrator automatically watches the config file for changes. To reload:

1. Edit `config/orchestrator.yaml`
2. Save the file
3. Watch logs for reload confirmation:
   ```json
   {
     "time": "2025-11-06T15:30:00",
     "level": "INFO",
     "msg": "config reloaded",
     "changes": {
       "log_level": {"old": "info", "new": "debug"},
       "http_port": {"old": 8080, "new": 9090}
     }
   }
   ```

**Note**: Some changes (like `listen_port`) require a restart. The logs will indicate which settings require restart.

---

## HTTP Monitoring API

### Health Check

Query the health endpoint to check if the server is operational:

```bash
curl http://localhost:8080/health
```

**Response (healthy)**:
```json
{
  "status": "healthy",
  "uptime_seconds": 3600,
  "dfhack_connected": true,
  "dfhack_connection_time": "2025-11-06T10:30:00Z",
  "last_heartbeat": "2025-11-06T11:29:55Z",
  "timestamp": "2025-11-06T11:30:00Z"
}
```

**Response (degraded - DFHack disconnected)**:
```json
{
  "status": "degraded",
  "uptime_seconds": 1800,
  "dfhack_connected": false,
  "dfhack_connection_time": null,
  "last_heartbeat": null,
  "timestamp": "2025-11-06T11:30:00Z"
}
```

### Readiness Check

For Kubernetes probes or load balancer health checks:

```bash
curl http://localhost:8080/ready
```

**Response (ready - HTTP 200)**:
```json
{
  "ready": true,
  "checks": {
    "config_loaded": true,
    "tcp_listener": true,
    "http_server": true,
    "logger": true
  }
}
```

**Response (not ready - HTTP 503)**:
```json
{
  "ready": false,
  "checks": {
    "config_loaded": true,
    "tcp_listener": false,
    "http_server": true,
    "logger": true
  },
  "message": "TCP listener failed to start on port 5001"
}
```

### Operational Metrics

Get current operational metrics:

```bash
curl http://localhost:8080/metrics
```

**Response**:
```json
{
  "server_uptime_seconds": 3600,
  "dfhack_connected": true,
  "session": {
    "connection_time": "2025-11-06T10:30:00Z",
    "messages_sent": 150,
    "messages_received": 148,
    "error_count": 2,
    "heartbeats_sent": 360,
    "heartbeats_received": 360
  },
  "config_reloads": 3,
  "http_requests": 1250,
  "timestamp": "2025-11-06T11:30:00Z"
}
```

---

## Performance Benchmarks

### Running Benchmarks

Execute all benchmarks:

```bash
cd C:\Users\zmanl\Projects\DF-AI
go test -bench=. ./tests/benchmark/...
```

Execute specific benchmark:

```bash
# Memory usage benchmarks
go test -bench=BenchmarkTileMemory ./tests/benchmark/

# Access speed benchmarks
go test -bench=BenchmarkTileAccess ./tests/benchmark/
```

### Benchmark Output

**Example output**:
```
BenchmarkTileMemory/Small_48x48x20-8              1000    1250000 ns/op    45 B/op    1 allocs/op
BenchmarkTileMemory/Medium_100x100x100-8           100   11500000 ns/op    58 B/op    1 allocs/op
BenchmarkTileMemory/Large_200x200x100-8             50   23000000 ns/op    61 B/op    1 allocs/op
PASS
```

**Interpreting results**:
- `1000`: Number of iterations run
- `1250000 ns/op`: Nanoseconds per operation
- `45 B/op`: Bytes allocated per operation (**target: ≤64 bytes**)
- `1 allocs/op`: Number of allocations per operation

**Access speed example**:
```
BenchmarkTileAccess/Small_48x48x20-8         5000000         280 ns/op     0 B/op    0 allocs/op
BenchmarkTileAccess/Medium_100x100x100-8     5000000         310 ns/op     0 B/op    0 allocs/op
BenchmarkTileAccess/Large_200x200x100-8      3000000         450 ns/op     0 B/op    0 allocs/op
PASS
```

**Interpreting access results**:
- `310 ns/op`: Nanoseconds per coordinate lookup (**target: ≤500ns**)
- `0 B/op`: No allocations during access (ideal)

### Benchmark Thresholds

Benchmarks automatically validate against these thresholds:

| Metric | Threshold | Why |
|--------|-----------|-----|
| Tile memory | ≤64 bytes/tile | Keeps 4M tiles under 256MB memory |
| Tile access | ≤500 ns/lookup | ~2 million lookups/second throughput |

**Failing benchmarks**:
If a benchmark exceeds thresholds, it will fail with:
```
FAIL: BenchmarkTileMemory/Large_200x200x100-8
      Expected ≤64 B/op, got 82 B/op
FAIL
```

### Running with Verbose Output

Get detailed statistics:

```bash
go test -bench=. -benchmem -benchtime=10s ./tests/benchmark/...
```

Options:
- `-benchmem`: Include memory allocation stats
- `-benchtime=10s`: Run each benchmark for 10 seconds (longer = more accurate)
- `-cpu=1,2,4,8`: Test with different CPU counts

---

## Integration with Monitoring Systems

### Prometheus (Future Enhancement)

The `/metrics` endpoint currently returns JSON. To integrate with Prometheus:

1. Use `json_exporter` to scrape JSON metrics
2. Future enhancement: Add Prometheus exposition format directly

**Example Prometheus config**:
```yaml
scrape_configs:
  - job_name: 'df-orchestrator'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
```

### Kubernetes Health Probes

**Liveness probe** (is server running?):
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 30
```

**Readiness probe** (is server ready for traffic?):
```yaml
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

### Load Balancer Health Checks

Configure load balancer to check `/ready` endpoint:
- **Path**: `/ready`
- **Port**: 8080
- **Success Code**: 200
- **Interval**: 10s
- **Timeout**: 5s
- **Unhealthy Threshold**: 3 consecutive failures

---

## Troubleshooting

### Config Not Reloading

**Issue**: Config file changes don't take effect

**Solutions**:
1. Check logs for validation errors:
   ```json
   {"level":"ERROR","msg":"config validation failed","error":"invalid port: 0"}
   ```
2. Ensure file is saved properly (not a temp file)
3. Some settings require restart - check logs for "requires restart" message

### HTTP Endpoints Not Responding

**Issue**: `curl: (7) Failed to connect to localhost:8080`

**Solutions**:
1. Check if HTTP API is enabled in config:
   ```yaml
   enable_http_api: true
   ```
2. Verify port in config matches curl command
3. Check logs for HTTP server startup message:
   ```json
   {"level":"INFO","msg":"HTTP server started","port":8080}
   ```

### Benchmarks Failing

**Issue**: Benchmarks exceed thresholds

**Solutions**:
1. Check if you're running on underpowered hardware
2. Close other CPU-intensive applications
3. Run with `-benchtime=30s` for more stable results
4. Review data structure implementation for optimization opportunities

---

## Next Steps

After completing this feature, you'll have:
- ✅ Externalized configuration with hot-reload
- ✅ Health monitoring endpoints
- ✅ Performance validation via benchmarks

**Ready for**: Implementing graph layers (Topology, Hazards, Traffic, Semantics) with validated infrastructure in place.
