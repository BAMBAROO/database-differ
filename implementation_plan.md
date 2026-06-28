# DB Schema Differ & Migrator — Project TODO

> **Bahasa:** Go (Golang)  
> **Platform:** macOS & Windows (cross-compile)  
> **Target DB:** MySQL & PostgreSQL  
> **Output:** CLI binary tunggal, tanpa dependency eksternal di target machine

---

## Gambaran Umum

Tool CLI yang membaca struktur dua database, membandingkan perbedaannya, menampilkan diff yang mudah dibaca, lalu menghasilkan migration SQL secara otomatis.

```
[Source DB] ──┐
               ├──► [Introspect] ──► [Diff Engine] ──► [Preview] ──► [Generate SQL] ──► [Apply?]
[Target DB] ──┘
```

---

## Phase 1 — Setup & Struktur Project

### 1.1 Inisialisasi Project

- [ ] Buat folder root project: `db-schema-differ/`
- [ ] Jalankan `go mod init` dengan nama module yang sesuai
- [ ] Tentukan struktur folder utama (lihat section 1.2)
- [ ] Buat file `README.md` awal dengan deskripsi singkat project
- [ ] Setup `.gitignore` untuk Go project (bin/, vendor/, \*.env)
- [ ] Buat file `.env.example` sebagai template konfigurasi koneksi

### 1.2 Struktur Folder

```
db-schema-differ/
├── cmd/
│   └── root.go              # Entry point CLI (cobra)
├── internal/
│   ├── connector/           # Logic koneksi ke database
│   ├── introspector/        # Baca metadata schema dari DB
│   ├── differ/              # Engine pembanding schema
│   ├── generator/           # Generator SQL migration
│   ├── reporter/            # Output: terminal, file, HTML
│   └── applier/             # Eksekusi SQL ke target DB
├── config/
│   └── config.go            # Parsing config dari env / flags / file
├── models/
│   └── schema.go            # Struct representasi schema (Table, Column, Index, dll)
├── Makefile                 # Build, test, cross-compile
├── .env.example
└── README.md
```

- [ ] Buat semua folder di atas
- [ ] Buat file `.go` placeholder kosong di tiap subfolder agar folder terbaca git

### 1.3 Dependencies yang Perlu Ditambahkan

- [ ] `github.com/spf13/cobra` — CLI framework
- [ ] `github.com/spf13/viper` — config management (env + file)
- [ ] `github.com/go-sql-driver/mysql` — MySQL driver
- [ ] `github.com/lib/pq` — PostgreSQL driver
- [ ] `github.com/fatih/color` — colorized terminal output
- [ ] `github.com/olekukonko/tablewriter` — tampilan tabel di terminal
- [ ] `github.com/schollz/progressbar` — progress bar saat introspect DB besar

---

## Phase 2 — Konfigurasi & Koneksi

### 2.1 Config System

- [ ] Tentukan semua parameter koneksi yang dibutuhkan:
    - `SOURCE_DSN` — DSN lengkap source database
    - `TARGET_DSN` — DSN lengkap target database
    - `DB_DRIVER` — `mysql` atau `postgres`
    - `OUTPUT_FORMAT` — `terminal`, `sql`, `html`, `json`
    - `OUTPUT_FILE` — path file output (opsional)
    - `AUTO_APPLY` — boolean, apakah langsung apply ke target
    - `DRY_RUN` — boolean, preview saja tanpa generate file
- [ ] Support tiga cara config (urutan prioritas):
    1. CLI flags (`--source-dsn=...`)
    2. Environment variables (`.env` file)
    3. Config file (`differ.yaml`)
- [ ] Validasi config saat startup — berikan error message yang jelas kalau ada yang kurang
- [ ] Sembunyikan password dari log output (masking `***`)

### 2.2 Database Connector

- [ ] Buat interface `Connector` yang bisa dipakai oleh MySQL maupun PostgreSQL
- [ ] Implementasi `MySQLConnector`:
    - [ ] Connect dengan retry logic (3x dengan backoff)
    - [ ] Validasi bahwa user punya privilege READ ke `INFORMATION_SCHEMA`
    - [ ] Ping test sebelum lanjut
- [ ] Implementasi `PostgreSQLConnector`:
    - [ ] Connect dengan retry logic (3x dengan backoff)
    - [ ] Validasi akses ke `pg_catalog` dan `information_schema`
    - [ ] Ping test sebelum lanjut
- [ ] Handle koneksi timeout — jangan hang tanpa batas
- [ ] Tutup koneksi dengan benar saat program selesai (`defer`)

---

## Phase 3 — Introspection (Baca Schema dari DB)

> Ini adalah inti dari tool — membaca "foto" struktur database.

### 3.1 Definisikan Data Models

> **Keputusan arsitektur:** Model menyimpan **normalized raw type string per dialect**, bukan unified type wrapper. Normalisasi dilakukan di introspector masing-masing sebelum data masuk ke model. Diff Engine hanya melakukan string comparison — kalau string sama berarti tidak berubah, kalau beda berarti MODIFIED. Cross-DB comparison (MySQL source vs PostgreSQL target) adalah fitur v2, bukan v1.

- [ ] Buat struct `Schema` — representasi satu database secara keseluruhan
- [ ] Buat struct `Table` dengan field:
    - Nama tabel
    - Engine (MySQL: InnoDB, MyISAM, dll)
    - Character set & collation
    - Comment tabel
    - Daftar kolom (`[]Column`)
    - Daftar index (`[]Index`)
    - Daftar foreign key (`[]ForeignKey`)
    - Daftar constraint (`[]Constraint`)
- [ ] Buat struct `Column` dengan field:
    - Nama kolom
    - `NormalizedType` string — tipe data yang sudah dinormalisasi per dialect (ini yang dipakai Diff Engine untuk comparison)
    - `RawType` string — tipe data asli dari DB (disimpan untuk keperluan debug dan SQL generation)
    - Panjang / presisi (misal: VARCHAR(255))
    - Nullable (YES/NO)
    - Default value
    - Auto increment
    - Character set & collation
    - Comment kolom
    - Posisi urutan kolom
- [ ] Buat struct `Index` dengan field:
    - Nama index
    - Tipe (PRIMARY, UNIQUE, FULLTEXT, INDEX)
    - Kolom yang di-index (bisa lebih dari satu)
    - Urutan kolom dalam composite index
- [ ] Buat struct `ForeignKey` dengan field:
    - Nama constraint
    - Kolom sumber
    - Tabel referensi
    - Kolom referensi
    - ON DELETE action
    - ON UPDATE action
- [ ] Buat struct `DiffResult` untuk menyimpan hasil perbandingan

### 3.2 MySQL Introspector

Query yang perlu diimplementasikan ke `INFORMATION_SCHEMA`:

- [ ] **Query tabel:** Ambil semua tabel di database beserta metadata-nya dari `INFORMATION_SCHEMA.TABLES`
- [ ] **Query kolom:** Ambil semua kolom per tabel dari `INFORMATION_SCHEMA.COLUMNS` beserta tipe, default, nullable, dsb
- [ ] **Query index:** Ambil semua index dari `INFORMATION_SCHEMA.STATISTICS`, group berdasarkan nama index
- [ ] **Query foreign key:** Ambil semua FK dari `INFORMATION_SCHEMA.KEY_COLUMN_USAGE` dan `INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS`
- [ ] **Query check constraint:** Ambil dari `INFORMATION_SCHEMA.CHECK_CONSTRAINTS` (MySQL 8.0+)
- [ ] Handle MySQL versi lama (5.7) yang belum punya beberapa tabel di `INFORMATION_SCHEMA`
- [ ] **Normalisasi tipe data MySQL** — dilakukan di sini sebelum data masuk ke struct `Column.NormalizedType`:
    - `int(11)` → `int`
    - `int(1)` → `int` _(bukan bool — biarkan tinyint(1) yang jadi bool)_
    - `tinyint(1)` → `tinyint(1)` _(pertahankan, ini idiom boolean MySQL)_
    - `varchar(N)` — pertahankan panjangnya karena panjang adalah bagian dari tipe
    - `integer` → `int`
    - Semua alias dikonversi ke canonical form sebelum disimpan

### 3.3 PostgreSQL Introspector

Query yang perlu diimplementasikan:

- [ ] **Query tabel:** Dari `information_schema.tables` filter `table_type = 'BASE TABLE'`
- [ ] **Query kolom:** Dari `information_schema.columns` beserta tipe, default, nullable
- [ ] **Query index:** Dari `pg_indexes` dan `pg_class`
- [ ] **Query foreign key:** Dari `information_schema.table_constraints` join `information_schema.key_column_usage`
- [ ] **Query sequence / serial:** Deteksi kolom SERIAL / BIGSERIAL / AUTO INCREMENT equivalent
- [ ] **Normalisasi tipe data PostgreSQL** — dilakukan di sini sebelum data masuk ke struct `Column.NormalizedType`:
    - `character varying(N)` → `varchar(N)`
    - `character(N)` → `char(N)`
    - `integer` → `int`
    - `boolean` → `boolean` _(pertahankan, berbeda dari MySQL `tinyint(1)`)_
    - `double precision` → `float8`
    - `timestamp without time zone` → `timestamp`
    - `timestamp with time zone` → `timestamptz`
- [ ] **Tidak ada cross-driver type mapping di v1** — MySQL dan PostgreSQL hanya bisa di-diff dengan sesama driver yang sama. Fitur cross-DB comparison (misal MySQL source vs PostgreSQL target) dijadikan v2 dengan tabel mapping yang bisa dikonfigurasi user.

### 3.4 Introspection Utilities

- [ ] Tampilkan progress bar saat introspect (berguna kalau DB punya ratusan tabel)
- [ ] Cache hasil introspect ke memory (jangan query ulang kalau dipakai berkali-kali)
- [ ] Log waktu yang dibutuhkan untuk introspect setiap DB

---

## Phase 4 — Diff Engine

> Jantung dari tool ini — algoritma perbandingan dua schema.

### 4.1 Table-Level Diff

- [ ] Deteksi **tabel baru** (ada di Source, tidak ada di Target)
- [ ] Deteksi **tabel yang dihapus** (ada di Target, tidak ada di Source)
- [ ] Deteksi **tabel yang berubah** — masuk ke column-level dan index-level diff

### 4.2 Column-Level Diff

Untuk setiap tabel yang ada di kedua DB:

- [ ] Deteksi **kolom baru** (ada di Source, tidak ada di Target)
- [ ] Deteksi **kolom yang dihapus** (ada di Target, tidak ada di Source)
- [ ] Deteksi **kolom yang berubah** — bandingkan field per field:
    - [ ] Tipe data berubah
    - [ ] Panjang / presisi berubah
    - [ ] Nullable berubah
    - [ ] Default value berubah
    - [ ] Auto increment berubah
    - [ ] Character set / collation berubah
    - [ ] Comment berubah
    - [ ] Posisi urutan kolom berubah (FIRST / AFTER)

### 4.3 Index-Level Diff

- [ ] Deteksi **index baru**
- [ ] Deteksi **index yang dihapus**
- [ ] Deteksi **index yang berubah** (kolom yang di-index berubah, tipe berubah)
- [ ] Handle PRIMARY KEY sebagai kasus khusus

### 4.4 Foreign Key Diff

- [ ] Deteksi **FK baru**
- [ ] Deteksi **FK yang dihapus**
- [ ] Deteksi **FK yang berubah** (referensi berubah, ON DELETE/UPDATE action berubah)

### 4.5 Logika Khusus

- [ ] **Rename detection (heuristik):** Kalau ada kolom yang dihapus dan kolom baru dengan tipe sama, flag sebagai _kemungkinan rename_ — tanya ke user apakah ini rename atau drop+add
- [ ] **Dependency ordering:** Pastikan urutan diff sudah benar:
    - Drop FK dulu sebelum drop kolom / tabel
    - Buat tabel dulu sebelum buat FK yang referensikan tabel itu
    - Drop index sebelum ubah kolom yang di-index
- [ ] **Circular FK detection:** Deteksi jika ada circular reference antar tabel dan berikan warning — namun **bukan error fatal**, karena circular FK tetap bisa ditangani oleh three-pass generator (lihat Phase 5.3)

### 4.6 Diff Result Structure

- [ ] Setiap item diff punya severity level:
    - `SAFE` — penambahan (kolom baru, tabel baru, index baru)
    - `WARNING` — perubahan yang bisa menyebabkan data loss (pemendekan VARCHAR, ubah tipe)
    - `DANGER` — penghapusan (drop kolom, drop tabel)
- [ ] Hitung summary: berapa tabel added/modified/removed, berapa kolom, dsb

---

## Phase 5 — SQL Generator

### 5.1 MySQL SQL Generator

> ⚠️ **Catatan penting — DDL Implicit Commit MySQL:** Di MySQL, semua statement DDL (`ALTER TABLE`, `CREATE TABLE`, `DROP TABLE`, dll) memicu **implicit commit** secara otomatis. Ini berarti MySQL **tidak mendukung transactional DDL** — jika ada 10 statement dan gagal di statement ke-5, 4 statement pertama sudah tersimpan permanen dan tidak bisa di-rollback. Berbeda dengan PostgreSQL yang mendukung full transactional DDL. Konsekuensi ini harus ditangani di Phase 7 (Applier) dengan mekanisme state file, bukan di level SQL Generator.

- [ ] Generate `CREATE TABLE` lengkap untuk tabel baru (dengan semua kolom, index, constraint)
- [ ] Generate `DROP TABLE` untuk tabel yang dihapus (dengan `IF EXISTS`)
- [ ] Generate `ALTER TABLE ... ADD COLUMN` untuk kolom baru (dengan posisi AFTER yang benar)
- [ ] Generate `ALTER TABLE ... DROP COLUMN` untuk kolom yang dihapus
- [ ] Generate `ALTER TABLE ... MODIFY COLUMN` untuk kolom yang berubah
- [ ] Generate `ALTER TABLE ... ADD INDEX` / `DROP INDEX` untuk index
- [ ] Generate `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` untuk FK baru
- [ ] Generate `ALTER TABLE ... DROP FOREIGN KEY` untuk FK yang dihapus
- [ ] **Tidak wrap dalam transaction** — tambahkan comment eksplisit di header file SQL:
    ```sql
    -- !! MySQL does not support transactional DDL !!
    -- !! If this script fails mid-way, changes CANNOT be rolled back !!
    -- !! Use the state file (--resume) to continue from where it failed !!
    ```
- [ ] Tambahkan comment di SQL: `-- Generated by db-schema-differ on <timestamp>`
- [ ] Tambahkan comment per section: `-- Table: users`, `-- Column changes`, dsb

### 5.2 PostgreSQL SQL Generator

- [ ] Generate `CREATE TABLE` dengan sintaks PostgreSQL
- [ ] Generate `ALTER TABLE ... ADD COLUMN`
- [ ] Generate `ALTER TABLE ... DROP COLUMN`
- [ ] Generate `ALTER TABLE ... ALTER COLUMN TYPE` (dengan USING clause kalau perlu)
- [ ] Generate `ALTER TABLE ... ALTER COLUMN SET NOT NULL` / `DROP NOT NULL`
- [ ] Generate `ALTER TABLE ... ALTER COLUMN SET DEFAULT` / `DROP DEFAULT`
- [ ] Generate `CREATE INDEX` / `DROP INDEX`
- [ ] Generate `ALTER TABLE ... ADD CONSTRAINT` / `DROP CONSTRAINT`
- [ ] Wrap dalam transaction PostgreSQL

### 5.3 Three-Pass Architecture & Ordering

> **Keputusan arsitektur:** SQL Generator menggunakan **tiga pass terpisah**, bukan satu pass linear. Ini adalah solusi permanen untuk circular FK — bukan workaround. Semua tabel dibuat dulu tanpa FK di Pass 1, baru FK ditambahkan di Pass 3. Topological sort tetap digunakan di Pass 1 untuk non-circular dependency, tapi circular FK tidak lagi menjadi masalah fatal.

**Pass 1 — Structure Pass** _(CREATE / DROP tabel tanpa FK)_

- [ ] DROP FK constraints (harus jalan dulu sebelum DROP tabel apapun)
- [ ] DROP tables
- [ ] CREATE tables — **tanpa FK constraint** (hanya kolom dan PRIMARY KEY)
- [ ] Urutan CREATE TABLE menggunakan topological sort berdasarkan dependency non-circular
- [ ] Jika terdeteksi circular FK saat sort, log warning dan lanjut — FK tetap aman karena dihandle di Pass 3

**Pass 2 — Column Pass** _(ALTER kolom di tabel yang sudah ada)_

- [ ] DROP indexes (sebelum modify kolom yang di-index)
- [ ] ADD COLUMNs (dengan posisi AFTER yang benar)
- [ ] MODIFY COLUMNs
- [ ] DROP COLUMNs

**Pass 3 — Constraint Pass** _(index dan FK — semua tabel sudah ada di sini)_

- [ ] ADD indexes
- [ ] DROP FK yang perlu diubah
- [ ] ADD FK constraints — semua tabel sudah ada, circular FK tidak jadi masalah
- [ ] ADD check constraints

**Aturan tambahan:**

- [ ] Pisahkan statement `DANGER` (DROP) ke bagian sendiri dengan komentar yang jelas
- [ ] Beri opsi flag `--safe-only` yang hanya generate statement `SAFE` (no DROP apapun)
- [ ] Setiap pass diberi header comment yang jelas di output SQL:
    ```sql
    -- ============================================================
    -- PASS 1: STRUCTURE (CREATE/DROP TABLES)
    -- ============================================================
    ```

---

## Phase 6 — Reporter / Output

### 6.1 Terminal Reporter (default)

- [ ] Tampilkan summary di bagian atas:

    ```
    Schema Diff Summary
    ───────────────────
    Source : mysql://dev-server/myapp (124 tables)
    Target : mysql://prod-server/myapp (121 tables)

    Tables  : +3 added  ~5 modified  -1 removed
    Columns : +12 added  ~8 modified  -2 removed
    Indexes : +4 added  ~0 modified  -1 removed
    ```

- [ ] Tampilkan detail per tabel dengan indentasi dan warna:
    - Hijau (`+`) untuk additions
    - Kuning (`~`) untuk modifications
    - Merah (`-`) untuk removals
- [ ] Untuk setiap kolom yang berubah, tampilkan before → after:
    ```
    ~ users.email
        type    : VARCHAR(100) → VARCHAR(255)
        nullable: YES → NO
    ```
- [ ] Tampilkan severity badge: `[SAFE]` `[WARNING]` `[DANGER]`
- [ ] Tampilkan warning count dan danger count di bagian akhir
- [ ] Support `--no-color` flag untuk output tanpa ANSI color (untuk CI/CD)

### 6.2 SQL File Output

- [ ] Simpan migration SQL ke file dengan nama otomatis: `migration_20250628_143022.sql`
- [ ] Atau simpan ke path yang ditentukan user via `--output` flag
- [ ] Pisahkan file menjadi dua kalau ada statement berbahaya:
    - `migration_safe.sql` — hanya ADD
    - `migration_breaking.sql` — MODIFY dan DROP

### 6.3 HTML Report (bonus)

- [ ] Generate HTML report self-contained (satu file, CSS inline)
- [ ] Tampilkan diff dengan syntax highlighting
- [ ] Bisa di-filter berdasarkan severity (toggle SAFE/WARNING/DANGER)
- [ ] Tampilkan SQL yang akan dijalankan di tiap section
- [ ] Cocok untuk dikirim ke tim / disimpan sebagai dokumentasi

### 6.4 JSON Output

- [ ] Export diff result sebagai JSON terstruktur
- [ ] Berguna untuk integrasi dengan tool lain atau CI/CD pipeline
- [ ] Struktur JSON: `{ "summary": {...}, "tables": { "added": [], "modified": [], "removed": [] } }`

---

## Phase 7 — Applier (Eksekusi SQL)

### 7.1 Interactive Mode

- [ ] Sebelum apply, tampilkan summary lengkap dan minta konfirmasi:

    ```
    ⚠️  WARNING: 2 DANGER statements detected (DROP operations)

    Continue and apply 47 statements to target database? [y/N]:
    ```

- [ ] Kalau ada statement DANGER, minta konfirmasi **dua kali** (`Type 'yes' to confirm:`)
- [ ] Tampilkan progress bar saat mengeksekusi statement satu per satu

### 7.2 Eksekusi & Error Handling

> **Dua code path berbeda berdasarkan driver** — dikontrol oleh `DB_DRIVER` di config. Ini bukan duplikasi, tapi memang perilaku yang berbeda secara fundamental antara MySQL dan PostgreSQL.

**Untuk PostgreSQL (Transactional DDL):**

- [ ] Jalankan semua statement dalam **satu transaction** (`BEGIN` / `COMMIT`)
- [ ] Kalau ada statement yang gagal, rollback seluruh transaction secara otomatis
- [ ] Tampilkan pesan sukses: _"All N statements applied successfully (rolled back on error)"_

**Untuk MySQL (Non-Transactional DDL — State File approach):**

- [ ] Sebelum eksekusi, buat file `migration_state.json` berisi semua statement dengan status `pending`:
    ```json
    {
        "generated_at": "2025-06-28T14:30:00Z",
        "driver": "mysql",
        "statements": [
            { "id": 1, "sql": "ALTER TABLE ...", "status": "pending" },
            { "id": 2, "sql": "CREATE TABLE ...", "status": "pending" }
        ]
    }
    ```
- [ ] Eksekusi statement **satu per satu**, update status ke `done` atau `failed` setelah setiap statement
- [ ] Kalau gagal di statement ke-N, hentikan eksekusi dan tampilkan pesan error yang jelas:

    ```
    ✗ Failed at statement #5 of 10
      SQL: ALTER TABLE orders ADD COLUMN ...
      Error: Duplicate column name 'status'

    ⚠️  4 statements have already been applied and CANNOT be rolled back (MySQL DDL)
    ✦  Run with --resume to continue from statement #5 after fixing the issue
    ✦  State file saved to: migration_state.json
    ```

- [ ] Support flag `--resume` — baca state file, skip semua yang sudah `done`, lanjut dari yang `failed`
- [ ] Tampilkan **warning besar** di awal kalau driver adalah MySQL:
    ```
    ⚠️  MySQL detected: DDL statements cannot be rolled back.
        A state file will be created to allow --resume on failure.
    ```

**Berlaku untuk keduanya:**

- [ ] Log setiap statement yang dieksekusi beserta hasilnya ke file log
- [ ] Setelah selesai, jalankan ulang introspect dan konfirmasi bahwa source == target
- [ ] Tampilkan berapa statement yang berhasil dijalankan

### 7.3 Dry Run Mode

- [ ] Flag `--dry-run`: tampilkan semua SQL yang AKAN dijalankan tapi tidak benar-benar eksekusi
- [ ] Tetap validasi koneksi ke kedua DB agar user tahu DSN-nya benar
- [ ] Cocok untuk review di CI/CD sebelum deploy ke production

---

## Phase 8 — CLI Interface

### 8.1 Command Structure

```
db-schema-differ [command] [flags]

Commands:
  diff      Tampilkan perbedaan antara dua schema
  generate  Generate SQL migration file
  apply     Apply perubahan ke target database
  version   Tampilkan versi tool

Global Flags:
  --source-dsn    DSN source database
  --target-dsn    DSN target database
  --driver        mysql | postgres (default: mysql)
  --config        Path ke config file (default: differ.yaml)
  --no-color      Disable colorized output
  --verbose       Tampilkan log detail

Apply-specific Flags:
  --dry-run       Preview SQL tanpa eksekusi
  --auto-confirm  Skip konfirmasi interaktif (untuk CI/CD)
  --safe-only     Hanya jalankan statement SAFE (no DROP)
  --resume        Lanjutkan dari state file yang tersimpan (MySQL only)
```

### 8.2 Contoh Penggunaan

- [ ] Pastikan semua command berikut berjalan dengan benar:

    ```bash
    # Lihat diff di terminal
    db-schema-differ diff --source-dsn="..." --target-dsn="..."

    # Generate SQL file
    db-schema-differ generate --output=migration.sql

    # Dry run
    db-schema-differ apply --dry-run

    # Apply langsung
    db-schema-differ apply --auto-confirm

    # Hanya tampilkan perubahan yang aman
    db-schema-differ diff --safe-only

    # Output JSON untuk CI/CD
    db-schema-differ diff --output-format=json

    # Resume setelah partial failure di MySQL
    db-schema-differ apply --resume --state-file=migration_state.json
    ```

### 8.3 Help & Error Messages

- [ ] Setiap command punya `--help` yang informatif
- [ ] Error message harus jelas dan actionable (bukan hanya stack trace)
- [ ] Contoh error yang baik:
    ```
    Error: Cannot connect to source database
    Reason: Access denied for user 'root'@'localhost'
    Fix: Check your SOURCE_DSN or ensure the user has SELECT privilege on INFORMATION_SCHEMA
    ```

---

## Phase 9 — Testing

### 9.1 Unit Test

- [ ] Test diff engine dengan schema yang dibuat secara manual (tanpa koneksi DB)
    - Test: kolom baru terdeteksi sebagai ADDED
    - Test: kolom hilang terdeteksi sebagai REMOVED
    - Test: tipe kolom berubah terdeteksi sebagai MODIFIED
    - Test: ordering statement (FK drop sebelum column drop)
    - Test: circular FK terdeteksi sebagai warning, bukan error fatal
    - Test: circular FK tetap bisa di-generate SQL-nya via three-pass (FK muncul di Pass 3)
- [ ] Test SQL generator — pastikan output SQL valid secara sintaks
    - Test: three-pass output benar (Pass 1 tidak ada FK, Pass 3 ada FK)
    - Test: MySQL output tidak ada `START TRANSACTION` / `COMMIT`
    - Test: PostgreSQL output dibungkus dalam transaction
- [ ] Test MySQL state file:
    - Test: state file dibuat sebelum eksekusi
    - Test: status di-update ke `done` setelah statement berhasil
    - Test: `--resume` skip statement yang sudah `done`
- [ ] Test config parsing — env vars, flags, dan config file
- [ ] Test rename heuristic detection
- [ ] Test normalisasi tipe data MySQL (`int(11)` → `int`, `character varying` → `varchar`)

### 9.2 Integration Test

- [ ] Setup dua database MySQL via Docker Compose untuk testing
- [ ] Setup dua database PostgreSQL via Docker Compose untuk testing
- [ ] Skenario test:
    - [ ] Dua schema identik → diff result harus kosong
    - [ ] Tambah tabel baru di Source → terdeteksi sebagai tabel ADDED
    - [ ] Hapus kolom di Source → terdeteksi sebagai kolom REMOVED
    - [ ] Ubah tipe kolom → terdeteksi sebagai MODIFIED dengan WARNING
    - [ ] Apply diff → jalankan ulang diff, hasilnya harus kosong

### 9.3 Manual Test Scenarios

- [ ] Test dengan DB kecil (< 10 tabel)
- [ ] Test dengan DB besar (> 100 tabel) — pastikan performance OK
- [ ] Test dengan schema yang punya circular FK — pastikan warning muncul dan SQL tetap di-generate
- [ ] Test dry run — pastikan tidak ada perubahan di DB
- [ ] Test `--no-color` untuk output CI-friendly
- [ ] Test MySQL partial failure simulation:
    1. Buat migration dengan 10 statement
    2. Buat salah satu statement sengaja error (misal kolom duplikat)
    3. Verifikasi state file tersimpan dengan status yang benar
    4. Perbaiki masalahnya, jalankan `--resume`
    5. Verifikasi hanya statement yang belum `done` yang dieksekusi

---

## Phase 10 — Build & Distribusi

### 10.1 Makefile

- [ ] Buat target `make build` — build untuk OS saat ini
- [ ] Buat target `make build-all` — cross-compile untuk semua platform dengan `CGO_ENABLED=0` **eksplisit**:
    ```makefile
    build-all:
        CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64  go build -o dist/differ-macos-amd64
        CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64  go build -o dist/differ-macos-arm64
        CGO_ENABLED=0 GOOS=windows GOARCH=amd64  go build -o dist/differ-windows-amd64.exe
        CGO_ENABLED=0 GOOS=linux   GOARCH=amd64  go build -o dist/differ-linux-amd64
    ```
    > ⚠️ **Wajib `CGO_ENABLED=0`:** Driver MySQL (`go-sql-driver/mysql`) dan PostgreSQL (`lib/pq`) keduanya pure Go dan tidak butuh CGO. Namun default CGO bisa berbeda per environment — jangan andalkan default. Tanpa flag ini, cross-compile dari macOS ke Windows bisa gagal atau menghasilkan binary yang butuh GCC di target machine. Jika suatu saat migrasi ke `pgx`, perhatikan bahwa beberapa extension pgx yang opsional bisa menarik CGO dependency.
- [ ] Buat target `make verify-cgo` — cek bahwa binary yang dihasilkan tidak ada dependency ke C library:
    ```makefile
    verify-cgo:
        @file dist/differ-windows-amd64.exe | grep -v "MS Windows" && echo "ERROR: unexpected file type" || true
        @go tool nm dist/differ-linux-amd64 | grep -i cgo && echo "ERROR: CGO found in binary" || echo "OK: no CGO"
    ```
- [ ] Buat target `make test` — jalankan semua unit test
- [ ] Buat target `make lint` — jalankan `golangci-lint`
- [ ] Buat target `make clean` — hapus folder `dist/`

### 10.2 Versioning

- [ ] Inject version string saat build via `-ldflags`:
    ```
    go build -ldflags="-X main.Version=1.0.0 -X main.BuildDate=2025-06-28"
    ```
- [ ] `db-schema-differ version` menampilkan versi dan tanggal build
- [ ] Ikuti Semantic Versioning (MAJOR.MINOR.PATCH)

### 10.3 Distribusi

- [ ] Upload binary ke GitHub Releases untuk setiap versi
- [ ] Sediakan install script untuk macOS/Linux (via `curl | bash`)
- [ ] Tambahkan ke README: instruksi install untuk macOS (via binary download) dan Windows (via binary download atau Scoop)

---

## Phase 11 — Dokumentasi

### 11.1 README.md

- [ ] Deskripsi singkat apa yang dilakukan tool ini
- [ ] Screenshot / demo GIF terminal output
- [ ] Quick start (install + contoh command paling dasar)
- [ ] Tabel semua flags yang tersedia
- [ ] Contoh output diff di terminal
- [ ] Contoh output SQL yang dihasilkan
- [ ] Keterangan supported DB dan versi minimumnya

### 11.2 Dokumentasi Tambahan

- [ ] `docs/config.md` — semua opsi konfigurasi lengkap
- [ ] `docs/sql-output.md` — penjelasan format SQL yang dihasilkan, three-pass architecture, dan urutan eksekusi
- [ ] `docs/mysql-limitations.md` — penjelasan khusus DDL implicit commit, state file, dan cara pakai `--resume`
- [ ] `docs/cicd-integration.md` — cara integrasi dengan GitHub Actions / GitLab CI
- [ ] `CHANGELOG.md` — catatan perubahan per versi

---

## Urutan Pengerjaan yang Disarankan

| Urutan | Phase                                                                        | Estimasi |
| ------ | ---------------------------------------------------------------------------- | -------- |
| 1      | Phase 1 — Setup & Struktur                                                   | 1-2 jam  |
| 2      | Phase 2 — Config & Koneksi                                                   | 2-3 jam  |
| 3      | Phase 3 — Introspection (MySQL dulu) + normalisasi tipe                      | 3-5 jam  |
| 4      | Phase 4 — Diff Engine (core logic + circular FK detection)                   | 4-6 jam  |
| 5      | Phase 5 — SQL Generator MySQL (three-pass architecture)                      | 4-5 jam  |
| 6      | Phase 6 — Terminal Reporter                                                  | 2-3 jam  |
| 7      | Phase 8 — CLI Interface + flag --resume                                      | 1-2 jam  |
| 8      | Phase 7 — Applier (dua code path: MySQL state file + PostgreSQL transaction) | 3-5 jam  |
| 9      | Phase 3 & 5 — PostgreSQL support                                             | 3-4 jam  |
| 10     | Phase 6 — HTML & JSON output                                                 | 2-3 jam  |
| 11     | Phase 9 — Testing (termasuk resume scenario)                                 | 4-5 jam  |
| 12     | Phase 10 & 11 — Build (CGO_ENABLED=0 + verify) & Docs                        | 2-3 jam  |

**Total estimasi: ~35-51 jam pengerjaan aktif**

---

> Dibuat untuk project **DB Schema Differ & Migrator** — Go CLI Tool  
> Cross-platform: macOS (amd64/arm64) & Windows (amd64)  
> Last updated: revisi arsitektur — MySQL DDL implicit commit, dialect-specific type normalization, three-pass SQL generator, CGO_ENABLED=0
