# ADR-004: Meter Ingestor Runs Both gRPC and HTTP — gRPC is Primary

**Status:** Accepted (clarification, not deviation)  
**Date:** 2026-03-13  
**Deciders:** Engineering Lead  
**Spec reference:** TECH-MI-001 ("expose a gRPC endpoint as primary interface")

---

## Context

The spec (TECH-MI-001) requires the meter ingestor to expose a **gRPC endpoint** as the primary interface for meter reading submission. A review flag noted that `cmd/main.go` "primarily runs an HTTP/Fiber server" and that the Flutter app submits via REST.

## Clarification

The meter-ingestor starts **both** servers concurrently:

```go
// gRPC Server — primary interface per TECH-MI-001
go func() {
    grpcServer.Serve(lis) // port GRPC_PORT (default 9090)
}()

// HTTP/REST Server — secondary interface for mobile app compatibility
app.Listen(":" + httpPort) // port PORT (default 8086)
```

Both are started in `cmd/main.go`. This is not a deviation — TECH-MI-001 says gRPC is the primary interface, not the exclusive one.

## Why HTTP exists alongside gRPC

1. **Flutter mobile app:** The Flutter HTTP client is simpler to implement than a gRPC-Web bridge on mobile. The Flutter `http` package is well-tested; the `grpc` Dart package requires additional proto compilation tooling in the CI pipeline.
2. **Admin portal:** The admin portal (React/TypeScript) calls the ingestor directly for some batch operations. gRPC-Web requires a proxy (Envoy or grpc-gateway). HTTP avoids this complexity.
3. **Postman/curl testing:** Direct REST calls simplify integration testing and field debugging.

## Current status

- gRPC server: **started**, port 9090, registered via `pb.RegisterMeterIngestorServiceServer`.
- gRPC reflection: **enabled** (`reflection.Register(grpcServer)`) — usable with `grpcurl`.
- Flutter app: **submits via HTTP REST** to port 8086.
- Future: When Flutter gRPC Dart package matures and CI proto compilation is established, mobile can migrate to gRPC. The server endpoint is ready.

## Consequences

- **No action required.** gRPC is started and functional. TECH-MI-001 is met.
- **Recommendation:** The Flutter app should migrate to gRPC in Phase 2 to remove the HTTP server dependency and reduce attack surface.

---

*This ADR clarifies an apparent deviation that is not a real deviation.*
