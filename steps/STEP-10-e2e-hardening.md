# STEP 10 — End-to-End & Hardening

**Tujuan:** Memvalidasi seluruh lifecycle Ringmaster (`init` → `plan` → `approve` → `run` → `answer` → `status` → `usage`) dari awal sampai akhir menggunakan mock test server, memastikan sanitasi security/key, penanganan error ramah pengguna, dan dokumentasi open-source lengkap.

**Referensi PLAN.md:** Seluruh dokumen.

**Prasyarat implementasi:** Step 01–09.

## Checklist

- [ ] `tests/mockserver/`: Mock HTTP server (`net/http/httptest`) yang mensimulasikan endpoint OpenAI-compatible:
  - Skenario `happy`: Master merespons plan 3 task (T01 → T02, T03 paralel), worker merespons tool calls dan laporan COMPLETED.
  - Skenario `retry`: Worker gagal acceptance di attempt 1, sukses di attempt 2.
  - Skenario `blocked`: Worker merespons BLOCKED, lalu lanjut sukses setelah command `ringmaster answer`.
  - Skenario `budget`: Token melebihi limit sehingga eksekusi berstatus `PAUSED`.
  - Skenario `conflict`: Dua task memodifikasi baris yang sama, memicu `NEEDS_REVIEW` di merge queue.
- [ ] `tests/e2e/e2e_test.go`: End-to-end integration test yang mengeksekusi binary CLI Ringmaster langsung di temporary directory (`t.TempDir()`).
- [ ] Security & Error Hardening:
  - Sanitasi API Key: pastikan string API key tidak pernah bocor ke file `.ringmaster/state.json`, logs, transcript JSONL, atau output terminal.
  - Path Jail: `run_cmd` dan file tools tidak dapat keluar dari worktree.
  - Error messages informatif dengan saran tindakan (misal: "plan belum di-approve, jalankan `ringmaster plan <name>` terlebih dahulu").
- [ ] Dokumentasi Open Source:
  - `README.md` di root repo: Konsep filosofi (cheap-by-default control plane), instalasi single binary (`go install` atau release binary), panduan konfigurasi `ringmaster.toml` multi-provider, dan tutorial quickstart.
  - `docs/troubleshooting.md`: Panduan mengatasi `BLOCKED`, `NEEDS_REVIEW`, `PAUSED`, dan penanganan merge conflict.
  - `LICENSE`: Lisensi Open Source (MIT / Apache 2.0).

## Acceptance

```bash
go test -v -race ./...
go test -v ./tests/e2e/...
go vet ./...
./bin/ringmaster --help
```

Kriteria kualitas:
- Semua test e2e lulus 100% tanpa external network call.
- Zero API key leakage.
- Clean build single static binary.

## Out of scope

- Multi-host distributed orchestration.
- Web UI dashboard.

## Catatan Keputusan

- Pengujian e2e mengandalkan `httptest.Server` lokal agar deterministik, cepat, dan zero-cost.

## Status

_Diisi saat step dikerjakan._
