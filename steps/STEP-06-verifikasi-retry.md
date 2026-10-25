# STEP 06 — Verifikasi, Retry & Escalation

**Tujuan:** Setelah worker selesai, Ringmaster menjalankan acceptance criteria sendiri secara independen, lalu memutuskan `DONE`, retry, `BLOCKED`, atau `NEEDS_REVIEW` sesuai kebijakan.

**Referensi PLAN.md:** Bagian 4 (bagian Verification), 7 (alur status, retry, escalation).

**Prasyarat implementasi:** Step 02, 04, 05.

## Checklist

- [ ] `internal/verify/acceptance.go`: `RunAcceptance(ctx context.Context, task TaskSpec, worktree string, sandbox Sandbox) ([]CheckResult, error)`.
  - Menjalankan setiap perintah di `task.Acceptance` di dalam worktree.
  - `CheckResult`: `Command string`, `ExitCode int`, `StdoutTail string`, `StderrTail string` (dipotong di 4 KB).
  - Semua kriteria harus exit code 0 agar lolos.
- [ ] `internal/verify/decision.go`: fungsi murni `Decide(report WorkerReport, checks []CheckResult, attempt int, maxRetries int, budgetExceeded bool) Decision`.
  - `budgetExceeded` → `DecisionPAUSED`.
  - `report.Status == BLOCKED` → `DecisionBLOCKED`.
  - `report.Status == NEEDS_REVIEW` → `DecisionNEEDS_REVIEW`.
  - `COMPLETED` dan semua acceptance checks exit 0 → `DecisionDONE`.
  - `COMPLETED` tapi ada check gagal:
    - `attempt <= maxRetries` → `DecisionRETRY` (sertakan log error stderr/stdout ke prompt attempt berikutnya).
    - attempt habis → `DecisionNEEDS_REVIEW`.
- [ ] `internal/verify/loop.go`: orkestrasi per-task attempt loop:
  - Jalankan `RunAttempt`.
  - Jika hasil butuh verifikasi, jalankan `RunAcceptance`.
  - Panggil `Decide`.
  - Render `outputs/Txx.md` dengan status dan log `## Verification` yang diisi oleh Ringmaster.
  - Catat log audit ke `.ringmaster/logs/events.jsonl` (append-only).
- [ ] Unit test (dengan mock runner & mock acceptance):
  - Sukses attempt 1 -> `DONE`.
  - Gagal attempt 1 lalu sukses di attempt 2 -> `DONE` (attempt count = 2).
  - Gagal berulang hingga batas `maxRetries` -> `NEEDS_REVIEW`.
  - Status `BLOCKED` tidak mengurangi jatah retry.

## Acceptance

```bash
go test -v ./internal/verify/...
go vet ./...
```

## Out of scope

- Eksekusi paralel banyak task dalam DAG (Step 08).
- Eksekusi merge queue (dihubungkan di scheduler Step 08).

## Catatan Keputusan

- `Decide()` harus berupa pure function tanpa side effects agar seluruh cabang state machine retry dapat diuji secara deterministik.
- Status `COMPLETED` dari worker tidak pernah dianggap `DONE` sebelum acceptance criteria lolos verifikasi eksternal.
