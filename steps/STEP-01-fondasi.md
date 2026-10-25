# STEP 01 — Fondasi & Konfigurasi

**Tujuan:** Modul Go terinisialisasi, CLI entry point berbasis `cobra` berjalan, konfigurasi `ringmaster.toml` dimuat dan divalidasi dengan `go-toml/v2`, serta `ringmaster init <name>` membuat struktur project.

**Referensi PLAN.md:** Bagian 4 (struktur project), 8 (budget & estimasi token), 9 (konfigurasi providers & agents), 10 (teknologi).

## Checklist

- [ ] Inisialisasi `go.mod` (Go 1.27+) dengan module path `github.com/openclaw/ringmaster` (atau `ringmaster`).
- [ ] Dependensi utama: `github.com/spf13/cobra`, `github.com/pelletier/go-toml/v2`, `github.com/charmbracelet/lipgloss`.
- [ ] Struktur project Go standar:
  - `cmd/ringmaster/main.go`: entry point.
  - `internal/cli/`: root command dan subcommands (`init`, dll).
  - `internal/config/`: struct konfigurasi dan loader TOML.
- [ ] CLI `ringmaster --help` berjalan dan menampilkan deskripsi tool.
- [ ] `internal/config/config.go`:
  - Struct `Config` berisi `Providers map[string]ProviderConfig`, `Agents map[string]AgentConfig`, `Budget BudgetConfig`, `Sandbox SandboxConfig`, `Routing RoutingConfig`, `Estimates map[string]EstimateConfig`.
  - `BudgetConfig` berisi `MaxTokensPerProject` (default `2_000_000`) dan `MaxTokensPerTask` (default `300_000`). Tidak ada field harga atau USD.
  - `EstimateConfig` berisi `TokensIn [2]int` dan `TokensOut [2]int` (min, max). Validator: min ≤ max.
  - Default `sandbox.allow`: `["pytest", "ruff", "mypy", "python", "node", "npm", "go test", "git status", "git diff"]`.
- [ ] Loader mencari `ringmaster.toml` di direktori kerja atau root repo, lalu validasi. Error validasi menampilkan nama key dan alasannya.
- [ ] Tambahkan `ringmaster.example.toml` sebagai template (mengikuti contoh di PLAN.md Bagian 9).
- [ ] `ringmaster init <name>`:
  - membuat `projects/<name>/` dengan `BRIEF.md` (template kosong), `tasks/`, `outputs/`, `.ringmaster/logs/`.
  - membuat `repo/` dan menjalankan `git init -b main` jika belum ada.
  - gagal dengan error jelas jika `projects/<name>` sudah ada (tanpa flag `--force`).
- [ ] `.gitignore` mengabaikan `.worktrees/`, `.ringmaster/`, `ringmaster.toml`, binary executable.
- [ ] Test unit: loader valid, loader gagal pada field salah, `init` membuat struktur, `init` kedua kali gagal.

## Acceptance

```bash
go build -o bin/ringmaster ./cmd/ringmaster
./bin/ringmaster --help
./bin/ringmaster init demo
test -d projects/demo/repo/.git && test -f projects/demo/BRIEF.md
go test -v ./...
go vet ./...
```

## Out of scope

- Pemanggilan provider/LLM (Step 03).
- Skema task dan DAG (Step 02).

## Catatan Keputusan

- `projects/` berada di root direktori eksekusi Ringmaster, bukan di subfolder internal.
- `ringmaster init` memakai `git init -b main`.
