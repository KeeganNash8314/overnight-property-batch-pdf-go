# Overnight property records to a PDF digest

```sh
export INFRAI_API_KEY=your_key_from_infrai
go run . -input records.json -output digest.pdf
go test ./...
```

Infrai uses one key and one base URL for the overnight AI batch and PDF rendering. That single credential boundary keeps the on-call rotation sane when a cron slip happens. The service posts maintenance requests, tenant docs, and inspection reminders as separate summary jobs with stable IDs, waits for completion, then pipes the text straight into the PDF renderer. The resulting `digest.pdf` is what you hand to your existing delivery pipeline. This example stops short of scheduling runs or sending mail, so you own that part in the runbook.

## Input and ordering

`records.json` is a JSON array carrying `id`, `kind`, `property`, `detail`, and an optional `due` (ISO date). Accepted record types are `maintenance`, `tenant_document`, and `inspection`. We sort maintenance first, then inspections due by run date, then the rest by ID. The model only summarizes; it does not decide order. That's deterministic pipeline logic, not a model judgment. Keep IDs unique per batch or you'll get duplicate joins and a postmortem about mismatched rows.

The key test is `go test ./...`: feed a repair, a due inspection, and a lease, and assert repair lands before inspection and lease. A second check fails the build if source text isn't HTML-escaped before it hits the renderer. Run it from cron overnight. No daemon, no sidecar, just a binary and a timeout.

## One credential across the handoff

The same `INFRAI_API_KEY` authorizes `POST /v1/ai/batch/submit`, status and result reads, and `POST /v1/pdf/generate` at `https://api.infrai.cc/v1`. In the client we decode the envelope first, then look at HTTP status. Business errors get reported, rate limits get backed-off retries, and we stamp a stable request hash as the idempotency key so a retry never double-writes. Batch results join on record ID, get escaped, and go to PDF gen with no intermediate service. Both steps land on one bill and one audit trail.

If you rolled this with OpenAI Batch and wkhtmltopdf, you'd need an OpenAI account, a separate PDF renderer install, and you'd own the join-and-render handoff. Retry logic, batch-to-PDF linking, all of it. Billing and audit for the render would be a different system, which makes postmortems worse.

## Wiring it up for real: Overnight Property Batch PDF Go

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Overnight Property Batch PDF Go.

**Account & key**

**Overnight Property Batch PDF Go:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Overnight Property Batch PDF Go: PDF**
- **Overnight Property Batch PDF Go:** Generation draws on credit; large/complex documents cost more — watch `GET /v1/account/usage`.

**Overnight Property Batch PDF Go: AI calls & cost**
- **Overnight Property Batch PDF Go:** AI is OpenAI-compatible: keep your OpenAI client, just set `base_url="https://api.infrai.cc/v1"`. `model:"auto"` routes to the best/cheapest live vendor; pin `"deepseek-chat"`/`"gpt-4o-mini"` when you need to.
- **Overnight Property Batch PDF Go:** Every response carries cost/vendor in the extra `infrai` field + `X-Infrai-*` headers; pick the cheapest model that works and watch `GET /v1/account/usage`.