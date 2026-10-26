# Ringmaster Examples

Koleksi template dan contoh proyek nyata yang siap di-init menggunakan Ringmaster.

---

## 1. `todo-app` (Go CLI Todo Application)
Contoh proyek pemula yang ideal untuk menguji alur kerja end-to-end:
- **Spesifikasi:** `examples/todo-app/BRIEF.md`
- **Tugas yang akan di-generate Master:**
  - `T01`: Core Todo struct and storage functions (`todo.go`)
  - `T02`: Unit tests and edge-case testing (`todo_test.go`)
  - `T03`: CLI interactive runner (`main.go`)

### Cara Menjalankan Uji Coba:

#### Skenario A: Init dengan nama project baru
```bash
bin/ringmaster init todo-demo
cp examples/todo-app/BRIEF.md projects/todo-demo/BRIEF.md
bin/ringmaster plan todo-demo
bin/ringmaster run todo-demo
```

#### Skenario B: Init langsung di folder yang sudah ada (blank init)
```bash
mkdir -p /tmp/my-blank-todo && cd /tmp/my-blank-todo
/home/ubuntu/project/ringmaster/bin/ringmaster init
# Otomatis menginisialisasi folder aktif saat ini sebagai Ringmaster project!
```
