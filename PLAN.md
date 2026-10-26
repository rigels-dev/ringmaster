# Ringmaster — Local AI Agent Orchestration Tool

> **Status:** Active Architecture Spec (v0.4 — Go Tech Stack & Provider-Agnostic Registry)
> **Last Updated:** 2026-10-25
> **Langkah implementasi:** lihat [`steps/README.md`](steps/README.md)

---

## 1. Problem Statement & Philosophy
Menggunakan model mahal (Claude Opus / Sonnet / GPT-4o) untuk seluruh siklus pengerjaan project menguras token dengan cepat.
Ringmaster adalah **Control Plane (Kube-Master)** open source berbasis single binary Go yang bertindak sebagai orkestrator task independen:

- **Provider-Agnostic:** Mendukung provider OpenAI-compatible apapun (Ollama, vLLM, OpenRouter, 9router, OpenAI langsung, LiteLLM) yang didaftarkan langsung oleh user di konfigurasi.
- **Ringmaster:** Planner (Master) + scheduler + verifier. Memecah task, menjadwalkan worker, memverifikasi hasil, dan mencatat usage token.
- **Cheap-by-default:** Master (model cerdas) memecah tugas; Worker (model murah) mengeksekusi dengan konteks terisolasi.

### Non-goals (MVP)
- Bukan IDE, bukan UI web. CLI single binary + file markdown saja.
- Bukan orkestrasi multi-mesin. Semua berjalan di satu host.
- Bukan pengganti review manusia. Hasil `NEEDS_REVIEW` selalu sampai ke user.
- **Bukan penghitung biaya USD.** Harga token berbeda antar provider, jadi Ringmaster hanya mencatat jumlah token. Konversi ke biaya dilakukan user sendiri.

---

## 2. Core Architecture & Roles

### A. Master Agent (Control Plane)
1. **Brief Ingestion:** Membaca `BRIEF.md`, batasan teknis, dan scope.
2. **Architecture & Stack Proposal:** Mengusulkan stack dan struktur direktori (user dapat override).
3. **Task Decomposition:** Memecah project menjadi DAG task (`plan.json`) dengan `depends_on`, estimasi token, dan worker yang disarankan.
4. **Scheduling:** Task independen berjalan paralel (pool dengan batas `max_parallel`). Task dependen menunggu upstream `DONE`.
5. **Verification & Merge:** Menjalankan acceptance criteria sendiri (bukan mempercayai laporan worker), lalu meng-merge hasil lewat antrean merge.
6. **Retry & Escalation:** Mengikuti kebijakan di Bagian 7.

### B. Worker Agents
Worker adalah **agen dengan loop tool**, bukan sekadar satu panggilan LLM. Ia harus bisa membaca dan menulis kode.

1. **Context Isolation:** Worker hanya menerima `BRIEF.md`, `tasks/Txx.md`, dan interface target. Tidak menerima riwayat task lain.
2. **Tools (fixed, lihat Bagian 6):** `read_file`, `write_file`, `list_dir`, `search`, `run_cmd`.
3. **Workspace:** Setiap task bekerja di `git worktree` sendiri (lihat Bagian 5).
4. **Report:** Saat selesai, worker menulis `outputs/Txx.md` dengan format di Bagian 4.

---

## 3. Human-in-the-Loop Approval Gate
Sebelum eksekusi berjalan, Master menyajikan proposal interaktif:

- Daftar task, type, dependencies (paralel / sequential), dan estimasi token **dalam bentuk rentang** (min–max) untuk input dan output.
- Suggested worker + alasan.

User dapat:
- `[y] Approve & Execute`: membuat `plan.approved.json` dengan hash plan.
- `[a] Reassign Worker` (`ringmaster assign T02 <alias>`)
- `[e] Edit Stack/Scope`: mengubah `plan.json`, lalu **approval dibatalkan** dan user harus approve ulang.

Eksekusi hanya boleh berjalan jika hash `plan.json` sama dengan hash di `plan.approved.json`.

---

## 4. Project File Conventions

```
projects/<name>/
├── BRIEF.md                  # tujuan, stack, konvensi (~1 halaman)
├── plan.json                 # DAG hasil planning (belum di-approve)
├── plan.approved.json        # DAG + hash yang sudah di-approve
├── tasks/
│   └── T01-<slug>.md         # spesifikasi task (frontmatter + scope)
├── outputs/
│   └── T01.md                # laporan worker (frontmatter + ringkasan)
├── repo/                     # git repo hasil project (dibuat oleh `ringmaster init`)
├── .worktrees/               # worktree per task (dikelola Ringmaster)
└── .ringmaster/
    ├── state.json            # status task, attempt, usage token (atomic write)
    └── logs/                 # transcript per attempt
```

**Tidak ada `BLOCKED.md` global.** Pertanyaan worker disimpan di `outputs/Txx.md` dengan `status: BLOCKED`, sehingga beberapa task bisa blocked bersamaan.

### Task spec (`tasks/T01-<slug>.md`)
```markdown
---
id: T01
title: Implementasi login JWT
type: backend            # backend | frontend | test | docs | infra
depends_on: []
worker: coder-cheap      # opsional; default dari routing
max_retries: 2           # default 2
acceptance:              # setiap perintah harus exit 0 di worktree task
  - "pytest tests/test_auth.py -q"
  - "ruff check src/auth"
---
## Scope
...
## Inputs / Interfaces
...
## Human Answers          # diisi via `ringmaster answer`, dibaca worker saat resume
```

### Report (`outputs/T01.md`)
```markdown
---
id: T01
status: COMPLETED        # COMPLETED | BLOCKED | NEEDS_REVIEW (self-report worker)
attempts: 1
calls: 14                # jumlah pemanggilan model di seluruh attempt
tokens_in: 12034
tokens_out: 2210
files_changed: [src/auth/jwt.py, tests/test_auth.py]
---
## Summary
...
## Verification (diisi Ringmaster, bukan worker)
- `pytest tests/test_auth.py -q` → exit 0
- `ruff check src/auth` → exit 0
## Blockers
...               # wajib diisi jika status BLOCKED: tulis pertanyaan konkret
```

Bagian **Verification** diisi oleh Ringmaster dari hasil menjalankan acceptance criteria. Klaim worker tidak dianggap bukti.

---

## 5. Workspace & Merge Queue (menutup celah konflik paralel)

- Project wajib memiliki `repo/` sebagai git repo. `ringmaster init` menjalankan `git init` jika belum ada.
- Setiap task mendapat worktree: `.worktrees/Txx` di branch `rm/Txx` dari `main` (atau dari commit upstream jika ada `depends_on`).
- Task dependen dimulai dari branch hasil upstream, sehingga kode upstream sudah tersedia.
- Setelah task `DONE`, Master memasukkan branch-nya ke **merge queue** (berurutan, satu per satu) ke `main`.
- Konflik merge → task menjadi `NEEDS_REVIEW` dengan daftar file konflik. Tidak ada auto-resolve di MVP.
- Rollback: cukup `git revert` pada commit merge task tersebut.

Dua task paralel yang menyentuh file yang sama tidak saling mengganggu saat bekerja, dan konflik hanya terdeteksi di merge queue.

---

## 6. Worker Runtime & Sandbox

**Tools yang tersedia untuk worker (tetap, tidak bisa ditambah dari task):**

| Tool | Perilaku |
|---|---|
| `read_file(path)` | Membaca file dalam worktree |
| `write_file(path, content)` | Menulis file dalam worktree |
| `list_dir(path)` | Daftar isi direktori |
| `search(pattern)` | Pencarian teks (ripgrep) dalam worktree |
| `run_cmd(cmd)` | Menjalankan perintah dari allowlist saja |

**Aturan sandbox:**
- Semua path di-resolve relatif ke worktree. Path yang keluar dari worktree (`..`, symlink keluar, path absolut) ditolak.
- `run_cmd` hanya menjalankan perintah yang cocok dengan allowlist di `ringmaster.toml` (default: `pytest`, `ruff`, `mypy`, `python`, `node`, `npm`, `git status`, `git diff`).
- Timeout per perintah: 120 detik. Output dipotong di 20 KB.
- Environment dibersihkan: API key dan variabel sensitif tidak diteruskan ke proses perintah.
- Tidak ada akses jaringan dari `run_cmd` di MVP (dicatat sebagai future work).

**Batas per attempt:** `max_turns` (default 30 tool-call loop) dan `max_tokens_per_attempt`. Melewati batas → attempt dianggap gagal.

---

## 7. Verification, Retry & Escalation (menutup celah kebijakan retry)

Alur per task:

```
PENDING → RUNNING (attempt n) → worker selesai
  ├─ worker status BLOCKED           → BLOCKED   (menunggu `ringmaster answer`)
  ├─ worker status NEEDS_REVIEW      → NEEDS_REVIEW
  └─ worker status COMPLETED
       → VERIFYING (jalankan acceptance)
          ├─ semua exit 0            → DONE → masuk merge queue
          └─ ada yang gagal
               ├─ attempt < max_retries+1 → RUNNING (attempt n+1), output gagal dikirim ke worker
               └─ habis                   → NEEDS_REVIEW
```

**Aturan:**
- `max_retries` default **2** (total maksimal 3 attempt).
- Output gagal (stderr/stdout terpotong) dikirim ke worker di attempt berikutnya.
- `BLOCKED` tidak dihitung sebagai retry. Task menunggu jawaban user, sementara task lain tetap berjalan.
- `NEEDS_REVIEW` selalu ditampilkan di `ringmaster status`.
- Task yang upstream-nya `NEEDS_REVIEW`, `BLOCKED`, atau `FAILED` berstatus `SKIPPED` (tidak dijalankan).

**Verify loop** (istilah pengganti "reconciliation"): membandingkan kondisi nyata (exit code acceptance) dengan kriteria, lalu memutuskan retry, blocked, atau done.

**Status task:** `PENDING`, `RUNNING`, `VERIFYING`, `DONE`, `BLOCKED`, `NEEDS_REVIEW`, `FAILED`, `SKIPPED`, `PAUSED`.

---

## 8. Token Usage & Budget

Ringmaster **tidak menghitung biaya dalam USD**. Yang dicatat adalah jumlah token. Alasannya: harga per token berbeda antar provider dan bisa berubah sewaktu-waktu. Konversi ke biaya bisa dilakukan user dari angka token dengan harga provider masing-masing.

### Pencatatan usage
- Setiap pemanggilan model mencatat `prompt_tokens` (tokens_in) dan `completion_tokens` (tokens_out) dari field `usage` respons provider. Sumber ini adalah kebenaran.
- Usage dikelompokkan per **role** (`master` / `worker`) dan per **agent alias**.
- Usage **planning** (pemanggilan Master di `ringmaster plan`) dicatat terpisah sebagai `planning`, tidak masuk ke task mana pun.
- Usage per task = jumlah semua attempt task tersebut (`calls`, `tokens_in`, `tokens_out`).
- Usage project = planning + semua task.

### Budget
- `[budget]` di `ringmaster.toml`: `max_tokens_per_project` dan `max_tokens_per_task` (total input + output).
- Budget project mencakup usage planning Master.
- Sebelum attempt dimulai, cek `project_tokens + max_tokens_per_attempt <= max_tokens_per_project`. Jika tidak, project berstatus `PAUSED` dan task baru tidak dimulai.
- Task yang melewati `max_tokens_per_task` berstatus `PAUSED`.
- Lanjut dengan `ringmaster resume <project>` setelah user menaikkan batas.

### Estimasi
- Estimasi per task diambil dari tabel `[estimates.<type>]` di `ringmaster.toml` berupa rentang token input dan output.
- Ditampilkan sebagai rentang (min–max) di preview plan. Tidak ada angka tunggal.
- Setelah project berjalan, `ringmaster usage` menampilkan rasio aktual/estimasi per type. Rasio ini dipakai user untuk mengkalibrasi tabel estimasi.

---

## 9. Providers & Agent Registry

- Gateway & Provider: kontrak **OpenAI-compatible** (`POST {base_url}/chat/completions`, dengan function calling untuk tools).
- User bebas mendaftarkan provider apa saja di `[providers.<name>]` dan memetakan agent ke provider tersebut.
- Konfigurasi di `ringmaster.toml`:

```toml
[providers.9router]
base_url = "http://localhost:20128/v1"
api_key_env = "NINEROUTER_API_KEY"

[providers.ollama_local]
base_url = "http://localhost:11434/v1"
api_key_env = "OLLAMA_API_KEY" # opsional/dummy

[providers.openrouter]
base_url = "https://openrouter.ai/api/v1"
api_key_env = "OPENROUTER_API_KEY"

[agents.master]
provider = "9router"
model = "claude-opus-5-5"
role = "master"

[agents.coder-cheap]
provider = "ollama_local"
model = "qwen2.5-coder:7b"
role = "worker"
tool_calling = true

[agents.coder-fast]
provider = "openrouter"
model = "anthropic/claude-3.5-haiku"
role = "worker"
tool_calling = true

[budget]
max_tokens_per_project = 2000000
max_tokens_per_task = 300000

[estimates.backend]
tokens_in = [20000, 80000]               # [min, max]
tokens_out = [5000, 25000]
```

- `ringmaster agents check` mengirim satu request uji dengan tool dummy ke masing-masing provider/model untuk memastikan dukungan tool calling. Alias yang gagal ditandai dan tidak boleh dipakai sebagai worker.
- Nilai budget dan estimasi di atas hanya contoh. Nilai yang sesuai ditentukan user.

---

## 10. Technical Requirements

- **Runtime:** Go 1.27+, single static binary, CLI-first, Goroutines + channels untuk eksekusi DAG paralel dan merge queue.
- **CLI Framework:** `spf13/cobra` + `spf13/viper`.
- **Styling / TUI:** `charmbracelet/lipgloss` (tabel DAG, status, log ringkas).
- **HTTP Client:** Lightweight wrapper berbasis Go standard `net/http` untuk kontrak OpenAI-compatible, zero external dependency bloat, penanganan retry exponential backoff (429/5xx), dan pembacaan token usage yang akurat.
- **Config & Serialization:** TOML via `pelletier/go-toml/v2`, JSON standard library.
- **Persistensi:** JSON dengan atomic write (tulis ke `.tmp`, lalu `os.Rename`). Tidak ada database di MVP.
- **Logging:** Transcript tiap attempt di `.ringmaster/logs/Txx-attemptN.jsonl`.

### CLI Commands

| Command | Fungsi | Tahap |
|---|---|---|
| `ringmaster init <name>` | Membuat struktur project, `repo/`, dan `git init` | 1 |
| `ringmaster agents list` | Daftar agent dari registry | 3 |
| `ringmaster agents check` | Uji tool calling tiap alias | 3 |
| `ringmaster plan <name>` | Master membuat `plan.json` dan menampilkan preview DAG | 7 |
| `ringmaster assign <task> <alias>` | Ganti worker sebuah task | 7 |
| `ringmaster run <name>` | Eksekusi sesuai DAG (setelah approval) | 8 |
| `ringmaster answer <task> --text "..."` | Menjawab BLOCKED, lalu task siap di-resume | 8 |
| `ringmaster resume <task\|project>` | Melanjutkan task `BLOCKED` atau project `PAUSED` | 8 |
| `ringmaster status <name>` | Pohon DAG live dengan status | 9 |
| `ringmaster usage <name>` | Total token per master dan per worker, per task, dan estimasi vs aktual | 9 |

---

## 11. Roadmap Singkat

Detail tiap langkah ada di `steps/`.

1. Fondasi & konfigurasi
2. Skema (task, report, DAG)
3. Gateway & agent registry
4. Worker runtime & sandbox
5. Workspace git & merge queue
6. Verifikasi, retry & escalation
7. Master planner & approval gate
8. Scheduler & state
9. Status & usage token
10. End-to-end & hardening

Tahap 1–6 cukup untuk satu worker sekuensial. Tahap 7–9 menambah DAG paralel dan gate interaktif.
