# STEP 08 — Scheduler, State & Pause/Resume

**Tujuan:** Menjalankan DAG yang sudah di-approve dengan pool paralel (Goroutines + Semaphore), menyimpan state atomic (`.ringmaster/state.json`), dan mendukung command `answer`, `resume`, serta handling pause karena limit budget.

**Referensi PLAN.md:** Bagian 2A (scheduling), 3 (approval), 5 (merge queue), 7 (status), 8 (budget token), 10 (CLI).

**Prasyarat implementasi:** Step 06 (verify loop), Step 07 (approval).

## Checklist

- [ ] `internal/scheduler/state.go`: Membaca dan menulis `.ringmaster/state.json` via atomic write (`WriteJSONAtomic`).
- [ ] `internal/scheduler/engine.go`: Engine penjadwalan DAG berbasis Goroutines & worker pool:
  1. Validasi `PlanHash` terhadap `plan.approved.json`. Jika mismatch, tolak eksekusi.
  2. Muat state. Task `DONE` di-skip.
  3. Gunakan semaphore channel `chan struct{}` dengan kapasitas `max_parallel`.
  4. Ambil task `PENDING` yang dependensinya sudah berstatus `DONE`.
  5. Setelah task berstatus `DONE`, masukkan branch ke `MergeQueue`. Jika merge conflict, tandai `NEEDS_REVIEW`.
  6. Task dengan upstream `NEEDS_REVIEW`, `BLOCKED`, atau `FAILED` otomatis ditandai `SKIPPED`.
- [ ] Budget check logic:
  - Sebelum attempt berjalan, pastikan total token project + perkiraan attempt tidak melebihi `max_tokens_per_project`. Jika lewat, ubah project status ke `PAUSED`.
  - Task yang melewati `max_tokens_per_task` otomatis diset ke `PAUSED`.
- [ ] CLI `ringmaster answer <task> --text "..."`:
  - Menulis teks jawaban ke bagian `## Human Answers` di file task markdown terkait.
  - Reset status task dari `BLOCKED` kembali ke `PENDING`.
- [ ] CLI `ringmaster resume <task|project>`:
  - Melanjutkan eksekusi task yang `BLOCKED` atau project yang `PAUSED` (setelah budget dinaikkan).
- [ ] Graceful shutdown:
  - Tangkap sinyal `os.Interrupt` / `SIGTERM` via `signal.NotifyContext`.
  - Simpan state yang sedang berjalan kembali ke `PENDING` agar dapat di-resume tanpa korupsi state.
- [ ] Unit test:
  - Eksekusi paralel task independen.
  - Dependensi upstream tertahan sampai upstream `DONE`.
  - Budget limit memicu `PAUSED`.
  - Graceful interrupt handling.

## Acceptance

```bash
go test -v ./internal/scheduler/...
go vet ./...
```

## Out of scope

- Live TUI progress tree (Step 09).
- Panggilan provider live di test (gunakan mock).

## Catatan Keputusan

- Task yang berstatus `RUNNING` di state saat inisialisasi (akibat sudden power off / kill) di-reset otomatis menjadi `PENDING` untuk re-attempt bersih.
