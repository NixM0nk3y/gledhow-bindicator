package telemetry

import (
	"strings"
	"testing"
)

func TestLog(t *testing.T) {
	ResetState()

	tests := []struct {
		name     string
		severity uint8
		msg      string
	}{
		{"debug message", SeverityDebug, "debug:test"},
		{"info message", SeverityInfo, "info:test"},
		{"warn message", SeverityWarn, "warn:test"},
		{"error message", SeverityError, "error:test"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ResetState()
			Log(tc.severity, tc.msg)

			logs := GetLogQueue()
			if len(logs) != 1 {
				t.Fatalf("expected 1 log, got %d", len(logs))
			}

			log := logs[0]
			if log.Severity != tc.severity {
				t.Errorf("severity = %d, want %d", log.Severity, tc.severity)
			}

			body := string(log.Body[:log.BodyLen])
			if body != tc.msg {
				t.Errorf("body = %q, want %q", body, tc.msg)
			}

			if log.Timestamp == 0 {
				t.Error("timestamp should not be zero")
			}
		})
	}
}

func TestLogConvenienceFunctions(t *testing.T) {
	tests := []struct {
		name     string
		logFunc  func(string)
		expected uint8
	}{
		{"LogDebug", LogDebug, SeverityDebug},
		{"LogInfo", LogInfo, SeverityInfo},
		{"LogWarn", LogWarn, SeverityWarn},
		{"LogError", LogError, SeverityError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ResetState()
			tc.logFunc("test message")

			logs := GetLogQueue()
			if len(logs) != 1 {
				t.Fatalf("expected 1 log, got %d", len(logs))
			}

			if logs[0].Severity != tc.expected {
				t.Errorf("severity = %d, want %d", logs[0].Severity, tc.expected)
			}
		})
	}
}

func TestLogQueueCircular(t *testing.T) {
	ResetState()

	// Fill queue beyond capacity (queue size is 8)
	for i := 0; i < 12; i++ {
		LogInfo("message")
	}

	logs := GetLogQueue()
	if len(logs) != 8 {
		t.Errorf("queue length = %d, want 8 (max)", len(logs))
	}
}

func TestLogTruncation(t *testing.T) {
	ResetState()

	// Message longer than 64 bytes
	longMsg := strings.Repeat("x", 100)
	LogInfo(longMsg)

	logs := GetLogQueue()
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}

	if logs[0].BodyLen != 64 {
		t.Errorf("bodyLen = %d, want 64 (truncated)", logs[0].BodyLen)
	}
}

func TestLogDisabled(t *testing.T) {
	ResetState()
	Disable()

	LogInfo("should not be queued")

	logs := GetLogQueue()
	if len(logs) != 0 {
		t.Errorf("expected 0 logs when disabled, got %d", len(logs))
	}

	Enable()
}

func TestLogWithTraceContext(t *testing.T) {
	ResetState()

	// Set trace context
	var traceID [16]byte
	var spanID [8]byte
	for i := 0; i < 16; i++ {
		traceID[i] = byte(i + 1)
	}
	for i := 0; i < 8; i++ {
		spanID[i] = byte(i + 10)
	}
	SetTraceContext(traceID, spanID)

	LogInfo("with trace")

	logs := GetLogQueue()
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}

	log := logs[0]
	if !log.HasTrace {
		t.Error("expected HasTrace = true")
	}

	if log.TraceID != traceID {
		t.Error("traceID mismatch")
	}

	if log.SpanID != spanID {
		t.Error("spanID mismatch")
	}
}

func TestRecordGauge(t *testing.T) {
	ResetState()

	RecordGauge("temperature", 25)

	metrics := GetMetricQueue()
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	m := metrics[0]
	name := string(m.Name[:m.NameLen])
	if name != "temperature" {
		t.Errorf("name = %q, want %q", name, "temperature")
	}

	if m.Value != 25 {
		t.Errorf("value = %d, want 25", m.Value)
	}

	if !m.IsGauge {
		t.Error("expected IsGauge = true")
	}

	if m.Timestamp == 0 {
		t.Error("timestamp should not be zero")
	}
}

func TestRecordCounter(t *testing.T) {
	ResetState()

	RecordCounter("requests.total", 100)

	metrics := GetMetricQueue()
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	m := metrics[0]
	name := string(m.Name[:m.NameLen])
	if name != "requests.total" {
		t.Errorf("name = %q, want %q", name, "requests.total")
	}

	if m.Value != 100 {
		t.Errorf("value = %d, want 100", m.Value)
	}

	if m.IsGauge {
		t.Error("expected IsGauge = false for counter")
	}
}

func TestMetricQueueCircular(t *testing.T) {
	ResetState()

	// Fill queue beyond capacity (queue size is 8)
	for i := 0; i < 12; i++ {
		RecordGauge("metric", int64(i))
	}

	metrics := GetMetricQueue()
	if len(metrics) != 8 {
		t.Errorf("queue length = %d, want 8 (max)", len(metrics))
	}

	// Oldest entries should be overwritten (values 0-3 gone, 4-11 remain)
	if metrics[0].Value != 4 {
		t.Errorf("oldest metric value = %d, want 4", metrics[0].Value)
	}
}

func TestMetricNameTruncation(t *testing.T) {
	ResetState()

	// Name longer than 32 bytes
	longName := strings.Repeat("x", 50)
	RecordGauge(longName, 42)

	metrics := GetMetricQueue()
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}

	if metrics[0].NameLen != 32 {
		t.Errorf("nameLen = %d, want 32 (truncated)", metrics[0].NameLen)
	}
}

func TestSpanLifecycle(t *testing.T) {
	ResetState()

	// Set trace context first
	var traceID [16]byte
	for i := 0; i < 16; i++ {
		traceID[i] = byte(i + 1)
	}
	SetTraceContext(traceID, [8]byte{})

	// Start span
	idx := StartSpanTest("test-operation")
	if idx < 0 {
		t.Fatal("StartSpanTest returned invalid index")
	}

	// Span should be active (not yet in completed list)
	spans := GetSpanQueue()
	if len(spans) != 0 {
		t.Errorf("expected 0 completed spans while active, got %d", len(spans))
	}

	// End span successfully
	EndSpan(idx, true)

	spans = GetSpanQueue()
	if len(spans) != 1 {
		t.Fatalf("expected 1 completed span, got %d", len(spans))
	}

	span := spans[0]
	name := string(span.Name[:span.NameLen])
	if name != "test-operation" {
		t.Errorf("span name = %q, want %q", name, "test-operation")
	}

	if !span.StatusOK {
		t.Error("expected StatusOK = true")
	}

	if span.StartTime == 0 {
		t.Error("StartTime should not be zero")
	}

	if span.EndTime == 0 {
		t.Error("EndTime should not be zero")
	}

	if span.EndTime < span.StartTime {
		t.Error("EndTime should be >= StartTime")
	}

	if span.TraceID != traceID {
		t.Error("traceID mismatch")
	}
}

func TestSpanFailedStatus(t *testing.T) {
	ResetState()
	SetTraceContext([16]byte{1, 2, 3}, [8]byte{})

	idx := StartSpanTest("failing-op")
	EndSpan(idx, false)

	spans := GetSpanQueue()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	if spans[0].StatusOK {
		t.Error("expected StatusOK = false for failed span")
	}
}

func TestSpanInvalidIndex(t *testing.T) {
	ResetState()

	// Should not panic with invalid index
	EndSpan(-1, true)
	EndSpan(100, true)

	spans := GetSpanQueue()
	if len(spans) != 0 {
		t.Errorf("expected 0 spans, got %d", len(spans))
	}
}

func TestSpanNameTruncation(t *testing.T) {
	ResetState()
	SetTraceContext([16]byte{1}, [8]byte{})

	longName := strings.Repeat("x", 50)
	idx := StartSpanTest(longName)
	EndSpan(idx, true)

	spans := GetSpanQueue()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	if spans[0].NameLen != 32 {
		t.Errorf("nameLen = %d, want 32 (truncated)", spans[0].NameLen)
	}
}

func TestDisabledMetrics(t *testing.T) {
	ResetState()
	Disable()

	RecordGauge("test", 42)

	metrics := GetMetricQueue()
	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics when disabled, got %d", len(metrics))
	}

	Enable()
}

func TestDisabledSpans(t *testing.T) {
	ResetState()
	Disable()

	idx := StartSpanTest("test")
	if idx != -1 {
		t.Errorf("StartSpanTest should return -1 when disabled, got %d", idx)
	}

	Enable()
}

func TestSeverityConstants(t *testing.T) {
	// Verify OTLP severity numbers match expected values
	if SeverityDebug != 5 {
		t.Errorf("SeverityDebug = %d, want 5", SeverityDebug)
	}
	if SeverityInfo != 9 {
		t.Errorf("SeverityInfo = %d, want 9", SeverityInfo)
	}
	if SeverityWarn != 13 {
		t.Errorf("SeverityWarn = %d, want 13", SeverityWarn)
	}
	if SeverityError != 17 {
		t.Errorf("SeverityError = %d, want 17", SeverityError)
	}
}

func TestSpanStatusConstants(t *testing.T) {
	// Verify OTLP status codes
	if SpanStatusUnset != 0 {
		t.Errorf("SpanStatusUnset = %d, want 0", SpanStatusUnset)
	}
	if SpanStatusOK != 1 {
		t.Errorf("SpanStatusOK = %d, want 1", SpanStatusOK)
	}
	if SpanStatusError != 2 {
		t.Errorf("SpanStatusError = %d, want 2", SpanStatusError)
	}
}
