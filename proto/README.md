# proto/ — the domain contract (buf)

Latent Frame's domain **models and events** as versioned protobuf, managed by
[buf](https://buf.build). This is the "microservices-ready" seam: today the generated
Go types flow through an in-process pipeline; when a stage splits into its own service,
the same protos define its Pub/Sub event payloads (and, if needed, its Connect RPC).

- `latentframe/v1/property.proto` — the pipeline models (`Property`, `Room`, `HeroFrame`,
  `GlobalVision`, `RestageBrief`, `Verdict`).
- `latentframe/v1/events.proto` — the domain events (`RoomRestaged`, `RoomVerified`, …),
  each the `data` payload of a [CloudEvent](https://cloudevents.io) whose `type` is a
  reverse-DNS name (`ai.latentframe.room.restaged.v1`).

## Generate

```bash
make proto        # buf lint + buf generate → gen/go/latentframe/v1
```

Generated code in [`gen/go`](../gen/go) is committed (so `go build` needs no codegen
step). Config: [`buf.yaml`](../buf.yaml), [`buf.gen.yaml`](../buf.gen.yaml).

## Messaging

Events are published through the `Publisher` interface in
[`pkg/events`](../pkg/events) — a **sync, in-process** implementation now; a **GCP
Pub/Sub** implementation later, behind the same interface. We build on the CloudEvents
standard rather than defining our own envelope. Connect/gRPC service definitions are
deferred until a stage actually needs a network RPC surface.
