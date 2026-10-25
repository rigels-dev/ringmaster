# STEP 05 — Workspace Git & Merge Queue

**Tujuan:** Setiap task bekerja di git worktree dan branch sendiri. Hasil yang sudah `DONE` masuk ke `main` lewat antrean merge (merge queue) yang teratur.

**Referensi PLAN.md:** Bagian 5 (workspace & merge queue).

**Prasyarat implementasi:** Step 01 (repo dibuat oleh `init`).

## Checklist

- [ ] `internal/workspace/git.go`: wrapper tipis di atas command `git` menggunakan `os/exec.CommandContext` dengan flag `-C <repo_path>` secara eksplisit.
- [ ] `CreateWorktree(projectRoot, taskID, baseRef string) (string, error)`:
  - branch `rm/Txx`, direktori `.worktrees/Txx`.
  - `baseRef` adalah `main` jika `depends_on` kosong, atau branch `rm/<upstream>` jika ada dependensi upstream.
  - Jika worktree sudah ada (misal setelah crash/resume), gunakan kembali tanpa error (idempotent).
- [ ] `RemoveWorktree(projectRoot, taskID string) error` dipanggil setelah task `DONE` dan berhasil di-merge.
- [ ] `internal/workspace/merge_queue.go`:
  - `MergeQueue` berbasis `sync.Mutex` sehingga hanya satu operasi git merge yang berjalan dalam satu waktu.
  - `Enqueue(taskID string) (*MergeResult, error)` menjalankan `git merge --no-ff rm/Txx` ke `main` di `repo/`.
  - Jika terjadi konflik: jalankan `git merge --abort`, lalu kembalikan `MergeResult{Conflict: true, ConflictedFiles: [...]}`.
- [ ] Unit test menggunakan temporary directory (`t.TempDir()`):
  - Dua branch independen merge tanpa konflik.
  - Dua branch conflicting memicu abort dan mendeteksi conflict tanpa merusak `main`.
  - Idempotensi `CreateWorktree`.

## Acceptance

```bash
go test -v ./internal/workspace/...
go vet ./...
```

## Out of scope

- Penjadwalan kapan merge dipanggil (Step 08).
- Auto-resolve merge conflict (di luar cakupan MVP).

## Catatan Keputusan

- Gunakan `git -C <repo>` eksplisit di setiap perintah git.
- Set `user.name="Ringmaster"` dan `user.email="ringmaster@local"` secara lokal jika repo belum memiliki identitas git.
