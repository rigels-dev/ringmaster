# STEP 09 — Status Tree & Usage Token

**Tujuan:** User dapat melihat pohon status DAG live di terminal dengan pewarnaan yang jelas, serta melihat ringkasan token usage yang akurat (per role, per agent, dan per task vs estimasi).

**Referensi PLAN.md:** Bagian 8 (usage & budget token), 10 (CLI `status` dan `usage`).

**Prasyarat implementasi:** Step 08.

## Checklist

- [ ] `internal/cli/status.go`: `ringmaster status <name>` merender visual tree DAG menggunakan `charmbracelet/lipgloss`.
  - Pewarnaan status: `DONE` (hijau), `RUNNING` (kuning), `BLOCKED` (oranye), `NEEDS_REVIEW` (merah), `SKIPPED` (abu-abu).
  - Menampilkan Task ID, Title, Attempts, TokensIn/TokensOut.
  - Ringkasan total task dan progress bar token budget project.
  - Flag `--watch`: refresh tampilan secara periodik (setiap 2 detik) hingga eksekusi selesai.
- [ ] Daftar actionable blocker: jika ada task `BLOCKED`, tampilkan kutipan `## Blockers` dan instruksi copy-paste command `ringmaster answer <task> --text "..."`.
- [ ] Daftar `NEEDS_REVIEW`: tampilkan alasan dan file `outputs/Txx.md`.
- [ ] `internal/cli/usage.go`: `ringmaster usage <name>`:
  - Tabel per-agent: Alias, Provider, Role, Calls, TokensIn, TokensOut, Total Tokens.
  - Usage Master Planning terpisah dari task execution.
  - Tabel per-task: Type, Worker, Attempts, Actual vs Estimated Tokens.
  - Rasio aktual vs estimasi per task type (untuk membantu user kalibrasi tabel `[estimates]`).
- [ ] Dukungan output `--json` untuk integrasi scripting / CI.
- [ ] Unit test:
  - Visual tree rendering pada status berbeda.
  - Perhitungan agregat token usage per-agent dan per-task.
  - Flag `--json` mengembalikan valid JSON payload.

## Acceptance

```bash
go test -v ./internal/cli/...
go vet ./...
```

## Out of scope

- Web UI atau dashboard browser.
- Notifikasi eksternal (Telegram, Discord, Slack webhook).
- Konversi token ke USD (Ringmaster murni mencatat token).

## Catatan Keputusan

- Command `status` dan `usage` hanya membaca data dari `.ringmaster/state.json` (read-only), tidak mengakibatkan lock dan aman dipanggil saat `ringmaster run` sedang aktif.
