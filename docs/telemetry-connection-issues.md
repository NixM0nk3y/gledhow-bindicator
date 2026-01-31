# OTLP Telemetry Connection Issues (Resolved)

This document records the connection issues encountered while implementing OTLP telemetry over the lneto TCP stack and how they were resolved.

## Environment

- **Device**: Raspberry Pi Pico 2 (RP2350)
- **Network Stack**: lneto (lightweight embedded TCP/IP)
- **WiFi Driver**: cyw43439
- **Target**: OTLP HTTP collector on port 4318
- **Scheduler**: TinyGo `-scheduler=tasks` (cooperative)

## Issue Summary

The telemetry module could send one HTTP POST request successfully, but subsequent requests failed with:

```
too many ongoing queries
```

## Root Cause

The error originates from lneto's **ARP handler** (`arp/handler.go:154`), not the TCP layer.

When `DialTCP()` is called for an address in the local subnet, the stack calls `arp.StartQuery()` to resolve the destination IP to a MAC address. However, these ARP queries are **never automatically cleaned up** after the TCP connection closes.

```go
// In lneto/x/xnet/stack-async.go DialTCP():
if s.subnet.Contains(addrp.Addr()) {
    mac = make([]byte, 6)
    ip := addrp.Addr().As4()
    err = s.arp.StartQuery(mac, ip[:])  // Query added, never removed
    // ...
}
```

The ARP handler has a fixed capacity (`MaxQueries`). Once full, `StartQuery()` returns "too many ongoing queries" and new connections fail.

**Why MQTT worked**: MQTT uses a single persistent connection, so only one ARP query is ever created.

## Solution

Call `DiscardResolveHardwareAddress6()` after closing each TCP connection to free the ARP query slot:

```go
// After connection close
conn.Close()
for i := 0; i < 10 && !conn.State().IsClosed(); i++ {
    time.Sleep(100 * time.Millisecond)
}
conn.Abort()

// Free ARP query slot
stack.DiscardResolveHardwareAddress6(collectorAddr.Addr())
```

## Other Issues Encountered

### Issue 1: No Data Transmitted

**Symptom**: TCP handshake completes but no HTTP data sent.

**Cause**: TinyGo cooperative scheduling - `Write()` queues data but stack needs CPU time to transmit.

**Fix**: Add `Flush()` + `Sleep()` after writes to yield to stack goroutine.

### Issue 2: JSON Truncation

**Symptom**: Server returns 400 Bad Request with "unexpected EOF".

**Cause**: TX buffer (1024 bytes) smaller than body size.

**Fix**: Increase TX buffer to 2560 bytes.

### Issue 3: Body Not Sent

**Symptom**: Headers sent but body missing.

**Cause**: Large writes overflow buffer before flush.

**Fix**: Write body in 1KB chunks with intermediate flushes.

## Final Configuration

```go
var (
    tcpRxBuf [512]byte
    tcpTxBuf [2560]byte
    BodyBuf  [2048]byte
    respBuf  [256]byte
)

const (
    FlushInterval = 30 * time.Second
    HTTPTimeout   = 10 * time.Second
    MaxRetries    = 2
)
```

## Key Lessons

1. **ARP and TCP are separate resources** - closing a TCP connection doesn't free the ARP query
2. **Check library source for error messages** - "too many ongoing queries" was in `arp/handler.go`, not TCP code
3. **Compare working vs broken code paths** - MQTT's single connection pattern avoided the ARP leak
