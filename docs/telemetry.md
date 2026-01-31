# Telemetry

The Bindicator includes an OpenTelemetry-compatible telemetry module for sending logs, metrics, and traces to an OTLP collector.

## Overview

The telemetry module is designed for embedded systems with:

- **Zero-heap design**: All buffers pre-allocated at compile time
- **OTLP/HTTP JSON format**: Compatible with OpenTelemetry Collector, Jaeger, Grafana Alloy, etc.
- **Queue-based async sending**: Non-blocking operation
- **Automatic slog bridge**: All application logs automatically queued for telemetry

## Configuration

Create `config/telemetry_collector.text` with your OTLP collector address:

```
192.168.1.100:4318
```

Port 4318 is the standard OTLP HTTP port. The module sends to:

| Endpoint | Data Type |
|----------|-----------|
| `/v1/logs` | Log records |
| `/v1/metrics` | Metrics (gauges and counters) |
| `/v1/traces` | Trace spans |

If the collector is not configured or unreachable, telemetry is disabled gracefully without affecting device operation.

## Console Commands

| Command | Description |
|---------|-------------|
| `telemetry` | Show telemetry status (enabled, queue sizes, sent counts, errors) |
| `telemetry-flush` | Force immediate flush of all queued data |

### Example Output

```
> telemetry
Telemetry Status:
  Enabled: true
  Collector: 192.168.1.100:4318
  Queued: logs=2 metrics=4 spans=1
  Sent: logs=156 metrics=312 spans=52
  Errors: 0
```

## OTLP Compatibility

The module produces standard OTLP JSON payloads compatible with any OpenTelemetry-compatible backend:

- **OpenTelemetry Collector**: Direct ingestion on port 4318
- **Jaeger**: Via OTLP receiver
- **Grafana Alloy**: Via OTLP receiver
- **Grafana Tempo**: Via OTLP endpoint
- **Honeycomb, Datadog, etc.**: Via their OTLP endpoints

### Resource Attributes

All telemetry includes these resource attributes:

| Attribute | Value |
|-----------|-------|
| `service.name` | `bindicator` |
| `service.version` | Build version |
| `host.name` | `bindicator` |

### Log Severity Levels

| Level | OTLP Number |
|-------|-------------|
| DEBUG | 5 |
| INFO | 9 |
| WARN | 13 |
| ERROR | 17 |

### Span Status Codes

| Status | OTLP Code |
|--------|-----------|
| Unset | 0 |
| OK | 1 |
| Error | 2 |

## Memory Budget

The telemetry module uses approximately 8.5KB of statically allocated memory:

| Component | Size |
|-----------|------|
| TCP Tx Buffer | 2.5KB |
| TCP Rx Buffer | 512B |
| Body Buffer | 2KB |
| Response Buffer | 256B |
| Log Queue (8 entries) | ~1.7KB |
| Metric Queue (8 entries) | ~400B |
| Span Queue (4 entries) | ~400B |
| Config/State | ~200B |
| **Total** | **~8.5KB** |

## Trace Context

The module generates trace IDs for each MQTT refresh cycle:

1. `GenerateTraceID()` creates a new trace context
2. `StartSpan("mqtt-refresh")` begins a span
3. All logs during the span are correlated via trace ID
4. `EndSpan(idx, success)` completes the span

This enables distributed tracing from the device through your backend.

## Background Sender

Telemetry data is sent asynchronously:

- **Flush interval**: Every 30 seconds
- **Timeout**: 10 seconds per HTTP request
- **Retries**: 2 attempts on failure
- **Non-blocking**: Main loop never waits for telemetry

## Running a Local Collector

### OpenTelemetry Collector

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318

exporters:
  logging:
    loglevel: debug

service:
  pipelines:
    logs:
      receivers: [otlp]
      exporters: [logging]
    metrics:
      receivers: [otlp]
      exporters: [logging]
    traces:
      receivers: [otlp]
      exporters: [logging]
```

```bash
docker run -p 4318:4318 \
  -v $(pwd)/otel-collector-config.yaml:/etc/otelcol/config.yaml \
  otel/opentelemetry-collector:latest
```

### Grafana Alloy

```alloy
otelcol.receiver.otlp "default" {
  http {
    endpoint = "0.0.0.0:4318"
  }
  output {
    logs    = [otelcol.exporter.logging.default.input]
    metrics = [otelcol.exporter.logging.default.input]
    traces  = [otelcol.exporter.logging.default.input]
  }
}

otelcol.exporter.logging "default" {}
```

## Testing with netcat

To verify the device is sending telemetry:

```bash
# Listen on OTLP port
nc -l 4318

# You should see HTTP POST requests like:
# POST /v1/logs HTTP/1.1
# Host: 192.168.1.100:4318
# Content-Type: application/json
# ...
```

## Disabling Telemetry

If no collector is configured, telemetry is automatically disabled. To explicitly disable:

1. Remove or empty `config/telemetry_collector.text`
2. Rebuild the firmware

The slog bridge continues to output logs to the serial console even when telemetry sending is disabled.
