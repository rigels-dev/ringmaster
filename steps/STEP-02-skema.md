# STEP 02 — Skema Task, Report, dan DAG

**Tujuan:** Semua format file (task spec frontmatter markdown, worker report, plan DAG, state) didefinisikan sebagai Go struct dengan parsing dan validasi yang ketat.

**Referensi PLAN.md:** Bagian 4 (format task dan report), 7 (status), 8 (usage token), 3 (hash approval).

## Checklist

- [ ] `internal/schemas/task.go`: `TaskSpec` (frontmatter `tasks/Txx-*.md`):
  - `ID` validasi regex `^T\d{2,3}$`.
  - `Type` enum: `backend`, `frontend`, `test`, `docs`, `infra`.
  - `DependsOn []string` (default empty), `Worker string`, `MaxRetries int` (default 2, rentang 0–5).
  - `Acceptance []string`, minimal 1 kriteria.
  - Parsing YAML frontmatter + markdown body; bagian `## Human Answers` diekstrak terpisah.
- [ ] `internal/schemas/report.go`: `WorkerReport` (frontmatter `outputs/Txx.md`):
  - `Status` enum: `COMPLETED`, `BLOCKED`, `NEEDS_REVIEW`.
  - `Attempts`, `Calls`, `TokensIn`, `TokensOut`, `FilesChanged []string`. Tanpa field harga USD.
  - Validasi: jika `Status == BLOCKED`, bagian `## Blockers` tidak boleh kosong.
  - Helper function untuk render report kembali ke markdown, termasuk inject hasil `## Verification`.
- [ ] `internal/schemas/usage.go`: struct `Usage` (`Calls`, `TokensIn`, `TokensOut`, method `TokensTotal()`) dan method `Add(other Usage)`.
- [ ] `internal/schemas/plan.go`: struct `Plan` dan `PlanTask`:
  - Validasi DAG: ID unik, semua `DependsOn` terdaftar, **deteksi siklus** menggunakan Kahn's algorithm (return error dengan detail node yang bersiklus).
  - Method `TopologicalLevels() [][]string`: mengembalikan level task untuk eksekusi paralel.
  - Method `PlanHash() (string, error)`: SHA-256 canonical JSON (keys terurut, whitespace konsisten).
- [ ] `internal/schemas/state.go`: struct `ProjectState` (PlanHash, TaskStates map[string]TaskState, Planning Usage, UsageByAgent, UsageByTask, UpdatedAt).
- [ ] `internal/storage/atomic.go`: fungsi `WriteJSONAtomic(path string, data any) error` (tulis ke `.tmp` lalu `os.Rename`).
- [ ] Unit test:
  - Task valid vs invalid (bad ID, empty acceptance, missing dependency).
  - Deteksi siklus (e.g. T01 -> T02 -> T01 dan self-cycle T01 -> T01).
  - Level topologis diamond DAG (T01 -> T02, T03 -> T04).
  - Hash deterministik dan stabil.
  - Atomic write test.

## Acceptance

```bash
go test -v ./internal/schemas/... ./internal/storage/...
go vet ./...
```

## Out of scope

- Eksekusi agent loop (Step 04).
- Pembuatan plan via Master LLM (Step 07).

## Catatan Keputusan

- Canonical JSON hash menggunakan `json.Marshal` dengan struct/map yang dinormalisasi untuk menjaga determinisme antar platform.
