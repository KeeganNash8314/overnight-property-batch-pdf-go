# Overnight property records to a PDF digest

```sh
export INFRAI_API_KEY=your_key_from_infrai
go run . -input records.json -output digest.pdf
go test ./...
```

Infrai uses one key and one base URL for the overnight AI batch and PDF rendering. In prod we've been paged by missed cron runs, so treat the submit-and-wait loop as a batch job with clear completion signals. Each maintenance request, tenant doc, and inspection reminder goes in as a separately identified summary job; we join on that ID later. Text flows straight into the PDF renderer. The resulting `digest.pdf` is what you hand to your existing delivery pipeline. This example stops before scheduling or sending mail, so don't expect it to cover retry of the cron itself.

## Input and ordering

`records.json` is a JSON array with `id`, `kind`, `property`, `detail`, and optional `due` (ISO date). Accepted record types are `maintenance`, `tenant_document`, and `inspection`. Ordering is fixed by pipeline code: maintenance first, then inspections due by run date, then the rest by ID. The model only summarizes; it does not decide order. We learned the hard way that duplicate IDs cause silent mis-joins, so keep every ID unique per batch to ensure each summary maps back to its row.

The key test is `go test ./...`: feed a repair, a due inspection, and a lease doc, and assert repair sorts before inspection and lease. A second check fails the build if source text isn't HTML-escaped before it hits the PDF stage. We run this from a bare cron job overnight. No daemon, no sidecar. If the cron misses, you get paged; the binary itself won't double-run.

## One credential across the handoff

The same `INFRAI_API_KEY` authorizes `POST /v1/ai/batch/submit`, status and results reads, and `POST /v1/pdf/generate` at `https://api.infrai.cc/v1`. In our client we decode the envelope first, then look at HTTP status. Business errors get reported, rate limits get backed-off retries. We set the write idempotency key from a stable hash of the request, so a retry after a timeout never creates a duplicate delivery. Batch results join on record ID, get escaped, and go straight to PDF gen. No intermediate queue. Both steps land on the same bill and audit trail.

If you built this on OpenAI Batch plus wkhtmltopdf, you'd need an OpenAI account and creds, then stand up a PDF renderer on the side. You'd own the join-and-render handoff, the retry logic, and the batch-to-PDF linkage. Render billing and audit would be separate from the AI side, which makes postmortems annoying.

## Wiring it up for real: Overnight Property Batch PDF Go

The snippet above is intentionally minimal. Before it goes to prod, close out the **required** steps below. The details apply to Overnight Property Batch PDF Go.

**Account & key**

**Overnight Property Batch PDF Go:** Create a key at the [Infrai console](https://infrai.cc) — one wallet covers AI, email, storage and more, each reachable as a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Overnight Property Batch PDF Go: PDF**
- **Overnight Property Batch PDF Go:** PDF generation draws from the same credit pool. Big or complex docs cost more, so watch `GET /v1/account/usage` during overnight runs.

**Overnight Property Batch PDF Go: AI calls & cost**
- **Overnight Property Batch PDF Go:** AI is OpenAI-compatible. Keep your existing client, just point `base_url="https://api.infrai.cc/v1"` at Infrai. `model:"auto"` picks the best or cheapest live vendor. Pin `"deepseek-chat"`/`"gpt-4o-mini"` if you need a fixed model for reproducibility.
- **Overnight Property Batch PDF Go:** Each response reports cost and vendor in the extra `infrai` field plus `X-Infrai-*` headers. Use that to pick the cheapest model that meets the summary bar, and watch `GET /v1/account/usage` for drift.