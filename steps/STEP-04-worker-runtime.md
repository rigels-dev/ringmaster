# STEP 04 — Worker Runtime & Sandbox

**Tujuan:** Satu attempt worker dapat dijalankan: loop percakapan dengan model, eksekusi tool (`read_file`, `write_file`, `list_dir`, `search`, `run_cmd`), dibatasi oleh sandbox, dan berhenti dengan `WorkerReport`.

**Referensi PLAN.md:** Bagian 2B (worker), 6 (tools & sandbox), 8 (batas token per attempt).

**Prasyarat implementasi:** Step 02 (schema), Step 03 (providers/client).

## Checklist

- [ ] `internal/worker/sandbox.go`:
  - `ResolvePath(root, rel string) (string, error)` menolak `..`, path absolut, dan symlink yang resolve keluar `root`.
  - `CheckCommand(cmd string, allowlist []string) bool` mencocokkan prefix token (bukan substring). `go test -v` cocok dengan `go test`; `rm -rf` tidak cocok dengan apa pun.
  - `ScrubbedEnv() []string` menghapus variabel environment yang mengandung `KEY`, `TOKEN`, `SECRET`, `PASSWORD`.
- [ ] `internal/worker/tools.go`: implementasi 5 tool native Go, mengembalikan string hasil atau pesan error untuk model.
  - `run_cmd`: gunakan `exec.CommandContext` (tanpa shell execution), `Dir=worktree`, timeout 120 detik, buffer dipotong 20 KB.
  - `search`: native regex directory search atau fallback ke `rg` jika tersedia.
- [ ] `internal/worker/runner.go`: `RunAttempt(ctx context.Context, task TaskSpec, worktree string, agent AgentConfig, budget BudgetConfig) (*AttemptResult, error)`.
  - System prompt menyertakan BRIEF.md, isi `tasks/Txx.md`, dan format `outputs/Txx.md`.
  - Loop tool-calling sampai `max_turns` (default 30) atau model berhenti.
  - Akumulasikan `Usage` (calls, tokens_in, tokens_out). Jika token melewati batas, hentikan attempt dengan alasan `BUDGET`.
  - Transcript attempt ditulis ke `.ringmaster/logs/Txx-attemptN.jsonl`.
- [ ] Jika worker tidak menulis laporan, runner membuat laporan fallback `NEEDS_REVIEW`.
- [ ] Unit test (dengan `httptest.Server` fake provider):
  - path traversal ditolak (`../../etc/passwd`).
  - `run_cmd` menolak perintah berbahaya (`rm -rf`, `sudo`).
  - loop berhenti saat max turns tercapai.
  - attempt berhenti saat budget limit tercapai.

## Acceptance

```bash
go test -v ./internal/worker/...
go vet ./...
```

## Out of scope

- Verifikasi acceptance criteria (Step 06).
- Git worktree (Step 05). Di step ini `worktree` cukup berupa direktori biasa.

## Catatan Keputusan

- `run_cmd` memakai `exec.CommandContext` langsung (bukan via `sh -c` atau `bash -c`), agar operator seperti `;`, `&&`, `|` tidak bisa menyelipkan arbitrary command.

