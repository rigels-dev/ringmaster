# Ringmaster — Troubleshooting & Operations Guide

Panduan penanganan status, bottleneck, dan intervensi operasional saat menjalankan orkestrasi Ringmaster.

---

## 1. Status Task `BLOCKED`

### Penyebab:
Worker membutuhkan informasi tambahan atau keputusan yang tidak dapat diambil secara mandiri (misal: kredensial environment khusus, konfigurasi port, atau izin arsitektur).

### Solusi & Tindakan:
1. Jalankan `ringmaster status` untuk membaca rincian blocker:
   ```bash
   ringmaster status <project_name>
   ```
2. Berikan jawaban atau kredensial yang diminta melalui command `answer`:
   ```bash
   ringmaster answer <task_id> --text "Database host is postgresql://user:pass@localhost:5432/db"
   ```
3. Lanjutkan orkestrasi:
   ```bash
   ringmaster resume <project_name>
   ```

---

## 2. Status Task `NEEDS_REVIEW`

### Penyebab:
- Terjadi **Merge Conflict** saat mencoba menggabungkan cabang task ke `main`.
- Worker gagal memenuhi acceptance criteria setelah batas `max_retries` terlampaui.

### Solusi & Tindakan:
1. Periksa laporan hasil worker di file:
   ```bash
   cat projects/<project_name>/outputs/<task_id>.md
   ```
2. Jika karena **Merge Conflict**:
   - Cabang `main` tetap bersih (Ringmaster otomatis menjalankan `git merge --abort`).
   - Masuk ke direktori repo dan selesaikan conflict secara manual, atau perbarui acceptance criteria.
3. Reset task ke pending atau update branch, lalu jalankan `ringmaster resume`.

---

## 3. Project Berstatus `PAUSED`

### Penyebab:
Total konsumsi token telah mencapai atau melebihi batas `budget.max_tokens_per_project` yang ditentukan di `ringmaster.toml`.

### Solusi & Tindakan:
1. Buka file `ringmaster.toml` dan naikkan batas budget:
   ```toml
   [budget]
   max_tokens_per_project = 3000000 # Naikkan sesuai kebutuhan
   ```
2. Lanjutkan eksekusi:
   ```bash
   ringmaster resume <project_name>
   ```

---

## 4. Plan Hash Mismatch ("Plan has been modified since approval")

### Penyebab:
File `plan.json` atau file task diubah secara manual setelah approval diberikan.

### Solusi:
Jalankan kembali command `ringmaster plan <project_name>` untuk meninjau perubahan dan berikan persetujuan ulang (`y`).
