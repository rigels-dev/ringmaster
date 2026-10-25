# Ringmaster — Langkah Implementasi

Spesifikasi lengkap ada di [`../PLAN.md`](../PLAN.md). Folder ini berisi urutan kerja yang bisa dieksekusi satu per satu.

## Aturan Umum

- Kerjakan step **berurutan**. Step N hanya dimulai jika step N-1 memenuhi *Acceptance*.
- Setiap step harus meninggalkan kode yang bisa dijalankan dan diuji.
- Jika ada keputusan yang tidak tercakup di PLAN.md, catat di bagian **Catatan Keputusan** pada step terkait, jangan diam-diam menyimpang.
- Test berada di package terkait atau `tests/` menggunakan standar Go testing (`go test ./...`). Provider atau LLM eksternal tidak boleh dipanggil di unit test (gunakan HTTP mock / test server).

## Definisi Selesai (global)

Sebuah step dianggap selesai jika:
1. Semua item di **Checklist** tercentang.
2. Semua perintah di **Acceptance** berhasil (exit 0).
3. `go test -v ./...` dan `golangci-lint run` (atau `go vet ./...`) tidak menghasilkan error.

Catatan: Ringmaster mencatat token, bukan biaya USD. Lihat PLAN.md Bagian 8.

## Daftar Step

| Step | Judul | Prasyarat |
|---|---|---|
| [01](STEP-01-fondasi.md) | Fondasi & konfigurasi | — |
| [02](STEP-02-skema.md) | Skema task, report, dan DAG | 01 |
| [03](STEP-03-gateway-agents.md) | Gateway client & agent registry | 01 |
| [04](STEP-04-worker-runtime.md) | Worker runtime & sandbox | 02, 03 |
| [05](STEP-05-workspace-git.md) | Workspace git & merge queue | 01 |
| [06](STEP-06-verifikasi-retry.md) | Verifikasi, retry & escalation | 02, 04, 05 |
| [07](STEP-07-planner.md) | Master planner & approval gate | 02, 03 |
| [08](STEP-08-scheduler.md) | Scheduler, state & pause/resume | 06, 07 |
| [09](STEP-09-status-usage.md) | Status tree & usage token | 08 |
| [10](STEP-10-e2e-hardening.md) | End-to-end & hardening | 09 |

**Milestone MVP sekuensial:** Step 01–06 selesai → satu worker bisa mengerjakan satu task dengan verifikasi dan retry.
**Milestone MVP penuh:** Step 01–10 selesai.
