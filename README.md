# 🎪 Ringmaster — Local AI Agent Orchestration Control Plane

> **Cheap-by-default, Provider-Agnostic, Multi-Agent Software Engineering Control Plane**

Ringmaster bertindak sebagai orkestrator (seperti *Kubernetes Control Plane*) untuk tim AI coding agents. Ringmaster memecah proyek software menjadi Directed Acyclic Graph (DAG) task, mendistribusikannya ke worker otonom di `git worktree` terisolasi, memverifikasi kriteria penerimaan secara mandiri, dan menggabungkan hasilnya ke branch `main` melalui antrean merge terserialisasi.

---

## 🌟 Fitur Utama

- **Provider-Agnostic:** Mendukung model OpenAI-compatible (Ollama, vLLM, OpenRouter, 9router, DeepSeek), Claude official API, dan Claude Subscription lokal (`~/.claude/` credentials via Claude CLI).
- **Steampunk / Circus Terminal TUI:** Setup interaktif 2 langkah dengan live provider search, badge visual master/worker, dan shortcut ping latency model (`p`).
- **Cryptographic Approval Gate:** Master AI menyusun rencana DAG dan estimasi min-max token. Eksekusi membutuhkan approval bertanda tangan canonical SHA-256 hash.
- **Strict Sandbox Security:** Path traversal blocker, command allowlist, dan environment scrubbing (zero API key leakage).
- **Autonomous Worker Loop:** Worker dibekali 5 native tools (`read_file`, `write_file`, `list_dir`, `search`, `run_cmd`) dengan timeout dan buffer truncation.
- **Independent Verification & Retry:** Master menjalankan acceptance test sendiri (tidak mempercayai laporan worker begitu saja). Jika gagal, otomatis di-retry dengan umpan balik error.
- **Serialized Git Merge Queue:** Setiap task bekerja di worktree terisolasi. Hasil yang lolos verifikasi di-merge ke `main` secara terserialisasi (otomatis abort jika ada konflik).
- **Human-in-the-Loop:** Command `answer` untuk membuka task yang terhenti (`BLOCKED`).

---

## 🚀 Panduan Memulai Cepat (Quickstart)

### 1. Build & Install
```bash
go build -o bin/ringmaster ./cmd/ringmaster
```

### 2. Konfigurasi Provider & Agent (TUI)
Jalankan konfigurasi berbasis terminal yang modern:
```bash
bin/ringmaster config
```
- Tekan `a` untuk menambah provider (OpenAI, Anthropic Claude CLI, Ollama, OpenRouter, dll).
- Sorot agent dan tekan `p` untuk menguji latency koneksi secara langsung.
- Tekan `m` untuk menetapkan Master Planner, dan `w` untuk Default Worker.

### 3. Inisialisasi Proyek Baru
```bash
bin/ringmaster init my-project
```
Perintah ini akan membuat direktori `projects/my-project/` dengan file `BRIEF.md` dan repository git kosong pada branch `main`.

### 4. Planning & Approval Gate
Edit `projects/my-project/BRIEF.md` sesuai spesifikasi aplikasi yang kamu inginkan, lalu minta Master AI menyusun DAG:
```bash
bin/ringmaster plan my-project
```
Tinjau tabel DAG dan estimasi token. Tekan `y` untuk menyetujui (`approve`).

### 5. Jalankan Orkestrasi
```bash
bin/ringmaster run my-project --parallel 4
```

### 6. Pantau Progress & Usage
```bash
# Pantau progress DAG secara live
bin/ringmaster status my-project --watch

# Analisis pemakaian token per agen & per task
bin/ringmaster usage my-project
```

---

## 📂 Struktur Direktori Proyek

```
projects/<project_name>/
├── BRIEF.md                 # Spesifikasi dan batasan proyek dari user
├── plan.json                # Rencana DAG task hasil generasi Master AI
├── plan.approved.json       # Rencana resmi yang disetujui user (canonical SHA-256)
├── tasks/                   # Spesifikasi detail tiap task (T01-*.md, T02-*.md)
├── outputs/                 # Laporan hasil verifikasi worker (T01.md, T02.md)
├── repo/                    # Repository git utama (branch 'main')
├── .worktrees/              # Worktree git terisolasi per task (dibuat dinamis)
└── .ringmaster/
    ├── state.json           # Status eksekusi & akumulasi token (atomic)
    └── logs/
        ├── events.jsonl     # Audit log append-only seluruh lifecycle
        └── Txx-attemptN.jsonl # Transkrip dialog & eksekusi tool per attempt
```

---

## 📜 Lisensi
Ringmaster dilisensikan di bawah lisensi [MIT](LICENSE).
