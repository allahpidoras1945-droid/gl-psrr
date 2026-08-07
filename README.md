# Glukoza

Concurrent B2B lead scraping, heuristic filtering, Telegram validation boundary, and export pipeline.

## Run

```powershell
go test ./...
go run ./cmd/app -input sources.txt -config configs/config.yaml
```

`config.yaml` is intentionally credential-free. Telegram validation remains `SKIPPED` until an MTProto session integration is configured. The default pipeline uses bounded HTTP reads, cancellation-aware workers, thread-safe deduplication, and JSON output.
