# STEP 07 — Master Planner & Approval Gate

**Tujuan:** Master agent membaca `BRIEF.md`, menghasilkan `plan.json` berupa DAG task beserta file `tasks/Txx-*.md`, menampilkan preview interaktif dengan estimasi token (min–max), dan memerlukan approval user sebelum dapat dieksekusi.

**Referensi PLAN.md:** Bagian 2A (master), 3 (approval gate), 8 (estimasi token).

**Prasyarat implementasi:** Step 02 (schema plan), Step 03 (providers & registry).

## Checklist

- [ ] `internal/planner/prompt.go`: Master system prompt yang menginstruksikan LLM memecah brief menjadi JSON terstruktur (`Plan`) dan mendeskripsikan tiap task spec lengkap.
- [ ] `internal/planner/generate.go`: `GeneratePlan(ctx context.Context, projectDir string, masterAgent AgentConfig, client Provider) (*Plan, error)`.
  - Memanggil agent dengan `role: "master"`.
  - Parse dan validasi JSON response dengan struct `Plan` (Step 02). Jika format salah, retry prompting 1 kali dengan pesan error parsing.
  - Tulis `plan.json` dan generate file markdown `tasks/Txx-*.md`.
  - Token usage proses planning dicatat ke `state.Planning` dan `state.UsageByAgent`.
- [ ] `internal/planner/estimate.go`:
  - Hitung estimasi rentang token (min-max) berdasarkan konfigurasi `estimates.<type>`.
  - Total rentang project dijumlahkan secara kumulatif (sum of min, sum of max).
- [ ] `internal/planner/preview.go`:
  - Render tabel terminal DAG menggunakan `lipgloss`: Task ID, Title, Type, Dependencies, Assigned Worker, Token Range.
  - Tampilkan budget warning jika total max token melebihi `budget.max_tokens_per_project`.
- [ ] CLI `ringmaster plan <name>`:
  - Generate/update plan dan tampilkan tabel preview.
  - Interactive gate CLI: prompt `[y] approve / [a] assign / [e] edit / [q] quit`.
  - `y`: Buat `plan.approved.json` dengan canonical hash `PlanHash`.
  - `a`: Ubah assigned worker (`ringmaster assign <task> <worker>`).
  - `e`: Buka `plan.json` di `$EDITOR`, re-validasi setelah disimpan, dan batalkan/hapus `plan.approved.json`.
- [ ] Verifikasi approval hash: Eksekusi `run` (Step 08) menolak jalan jika hash `plan.json` tidak identik dengan `plan.approved.json`.
- [ ] Unit test:
  - Preview menampilkan rentang (min-max).
  - Edit plan membatalkan approval.
  - Hash berubah jika ada task dimodifikasi.

## Acceptance

```bash
go test -v ./internal/planner/...
go vet ./...
```

## Out of scope

- Eksekusi runtime worker (Step 08).
- Multi-turn conversational planning (cukup 1-shot generation dengan schema repair retry).

## Catatan Keputusan

- Master mereturn raw JSON schema untuk DAG, dan deskripsi task ditulis ke file disk oleh Ringmaster.
