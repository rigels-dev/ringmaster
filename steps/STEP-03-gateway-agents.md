# STEP 03 — Providers, Gateway Client & Agent Registry

**Tujuan:** Ringmaster memiliki HTTP client OpenAI-compatible native Go yang fleksibel untuk memanggil provider LLM apapun (9router, Ollama, OpenRouter, direct provider), mendukung tool calling, dan mencatat token usage. Registry agen dapat diinspeksi dan diuji via CLI.

**Referensi PLAN.md:** Bagian 9 (providers & agent registry), 8 (usage token).

## Checklist

- [ ] `internal/llm/client.go`: `Client` berbasis standard Go `net/http.Client`.
  - Interface `Provider`: method `Chat(ctx context.Context, model string, req ChatRequest) (*ChatResponse, error)`.
  - Struct `ChatResponse`: `Content string`, `ToolCalls []ToolCall`, `FinishReason string`, `Usage TokenUsage` (`PromptTokens`, `CompletionTokens`).
  - API key dibaca dari environment variable yang dikonfigurasikan di provider (`api_key_env`). Jika tidak ada / kosong, kembalikan error deskriptif tanpa mencetak secret.
- [ ] Retry logic transparan: 2 kali percobaan dengan exponential backoff untuk HTTP status 429 dan 5xx. Langsung fail tanpa retry untuk 401/403/400.
- [ ] `internal/registry/registry.go`:
  - Membaca konfigurasi `[providers.*]` dan `[agents.*]`.
  - Menyediakan helper: `GetAgent(alias string)`, `ListWorkers()`, `ListMasters()`.
  - Memvalidasi bahwa setiap agent menunjuk ke provider yang terdaftar di konfigurasi.
- [ ] CLI `ringmaster agents list`:
  - Menampilkan tabel via `lipgloss`: Alias, Provider, BaseURL, Model, Role, Tool Calling support.
- [ ] CLI `ringmaster agents check`:
  - Mengirim request uji dengan tool dummy `ping(message: string)`.
  - Valid jika model mengembalikan `tool_calls` dengan function name `ping`.
  - Output status ditulis ke `.ringmaster/agent_check.json` beserta timestamp.
  - Model yang gagal ditandai `tool_calling: false` dan dilarang digunakan sebagai worker di runtime.
- [ ] Unit test menggunakan `httptest.Server`:
  - Respons chat normal.
  - Tool calling response parsing.
  - Skenario HTTP 429 lalu sukses (retry).
  - Skenario HTTP 401 (langsung error).

## Acceptance

```bash
go test -v ./internal/llm/... ./internal/registry/...
./bin/ringmaster agents list
go vet ./...
```

## Out of scope

- Worker loop dengan tool execution di local file system (Step 04).
- Prompting Master Planner (Step 07).

## Catatan Keputusan

- Client tidak memakai library pihak ketiga yang besar agar parsing token usage dan format tool call tetap presisi di seluruh variasi provider OpenAI-compatible.
- Token usage diambil langsung dari field response provider sebagai single source of truth.
