# proto/ — API contracts (placeholder)

buf + Connect/gRPC service definitions, generated into `gen/`. Introduced when there's a
real network boundary (e.g. the async ingest API: submit a video → poll job status by
id). Until then the pipeline is in-process Go and needs no proto.
