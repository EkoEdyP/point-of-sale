-- Skema database POS: single tenant, server lokal di LAN toko, satu database.
-- Butuh PostgreSQL 13+ (gen_random_uuid() bawaan). Uang disimpan sebagai integer Rupiah (bigint), tanpa desimal.

-- ---------- Fungsi bersama ----------

CREATE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Versi baris pre-order dan pengiriman naik setiap kali isi barisnya berubah. Formulir ubah mengirim versi yang
-- dibacanya, dan server menolak perubahan kalau barisnya sudah diubah orang lain sejak itu, supaya perubahan tidak
-- saling menimpa. UPDATE yang tidak mengubah apa-apa (mis. jadwal pengiriman yang ikut disamakan dengan
-- pre-order-nya) tidak menaikkan versi. updated_at diabaikan karena trigger set_updated_at selalu mengubahnya.
CREATE FUNCTION bump_version() RETURNS trigger AS $$
BEGIN
  IF to_jsonb(NEW) - 'version' - 'updated_at' IS DISTINCT FROM to_jsonb(OLD) - 'version' - 'updated_at' THEN
    NEW.version := OLD.version + 1;
  ELSE
    NEW.version := OLD.version;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Nomor dokumen: awalan, tanggal, lalu nomor urut minimal 6 digit. Setelah 999999 nomornya lanjut menjadi 7 digit,
-- karena lpad(..., 6) akan memotongnya sehingga nomornya sama dengan nomor lama dan penyimpanan gagal.
CREATE FUNCTION next_doc_no(prefix text, seq regclass) RETURNS text AS $$
  SELECT prefix || '-' || to_char(now(), 'YYYYMMDD') || '-' ||
         CASE WHEN s.n < 1000000 THEN lpad(s.n::text, 6, '0') ELSE s.n::text END
  FROM nextval(seq) AS s(n)
$$ LANGUAGE sql VOLATILE;

-- Nomor urut struk (INV), pre-order (PO), dan surat jalan (PG). Berlaku untuk semua hari, tidak mulai lagi tiap hari.
CREATE SEQUENCE receipt_seq;
CREATE SEQUENCE preorder_seq;
CREATE SEQUENCE delivery_seq;

-- ---------- Akun ----------

CREATE TABLE users (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name          text NOT NULL,
  username      text NOT NULL,
  password_hash text NOT NULL,                       -- bcrypt
  role          text NOT NULL CHECK (role IN ('admin', 'cashier')),
  active        boolean NOT NULL DEFAULT true,
  created_by    uuid REFERENCES users(id),           -- NULL untuk admin pertama (seed-admin)
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_username_lower_idx ON users (lower(username));

CREATE TRIGGER trg_users_updated BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Sesi login (refresh token) dengan rotasi. Yang disimpan hanya hash SHA-256 dari token acak, bukan tokennya.
-- Saat /auth/refresh token lama diberi revoked_at dan replaced_by = id token baru. Token yang sudah dicabut lalu
-- dipakai lagi dianggap dicuri, jadi semua sesi akun itu dicabut. Server menghapus token kedaluwarsa setiap hari;
-- token lama yang menunjuk penggantinya yang sudah terhapus cukup kehilangan tautannya.
CREATE TABLE refresh_tokens (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash  text NOT NULL UNIQUE,
  expires_at  timestamptz NOT NULL,
  revoked_at  timestamptz,
  replaced_by uuid REFERENCES refresh_tokens(id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);

-- ---------- Produk ----------

CREATE TABLE categories (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- kind: produk toko (store, dijual di kasir) atau produk pre-order (preorder, dibuat sesuai pesanan). Untuk produk
-- PO, stock berarti stok siap: hasil produksi yang belum diserahkan ke pelanggan.
-- Harga grosir (opsional): kalau jumlah satu produk dalam satu transaksi kasir atau satu pre-order mencapai
-- wholesale_min_qty, harga satuannya memakai wholesale_price, bukan price. wholesale_price NULL = tanpa harga
-- grosir. Harga grosir harus lebih murah dari harga jual, dan batasnya 2 sampai 10.000 pcs (jumlah maksimal per
-- produk dalam satu transaksi atau pre-order).
CREATE TABLE products (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  sku               text,
  name              text NOT NULL,
  category_id       uuid REFERENCES categories(id),
  price             bigint NOT NULL CHECK (price >= 0),
  stock             integer NOT NULL DEFAULT 0,      -- diubah dalam transaksi database yang sama dengan penjualan
  active            boolean NOT NULL DEFAULT true,
  deleted_at        timestamptz,                     -- soft delete
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  kind              text NOT NULL DEFAULT 'store' CHECK (kind IN ('store', 'preorder')),
  wholesale_price   bigint,
  wholesale_min_qty integer NOT NULL DEFAULT 50,
  CONSTRAINT products_wholesale_price_check
    CHECK (wholesale_price IS NULL OR (wholesale_price > 0 AND wholesale_price < price)),
  CONSTRAINT products_wholesale_min_qty_check CHECK (wholesale_min_qty BETWEEN 2 AND 10000)
);

-- SKU cukup unik di antara produk yang belum dihapus dalam satu jenis: produk yang sudah dihapus tidak menghalangi
-- pemakaian ulang SKU-nya, dan daftar produk toko dan PO terpisah.
CREATE UNIQUE INDEX products_sku_active_idx ON products (kind, sku) WHERE deleted_at IS NULL;
CREATE INDEX products_name_idx ON products (name);

CREATE TRIGGER trg_products_updated BEFORE UPDATE ON products
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------- Transaksi ----------

-- total = subtotal - discount + shipping_fee + other_fee, sama seperti yang dihitung server. Ongkir dan biaya lain
-- hanya terisi di struk penyerahan pre-order (penjualan toko selalu 0), dan down_payment adalah bagian total yang
-- sudah dibayar lewat DP pre-order. Struk batal selalu punya waktu, akun, dan alasan pembatalan; struk lain tidak
-- punya satu pun.
CREATE TABLE transactions (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  receipt_no     text NOT NULL UNIQUE DEFAULT next_doc_no('INV', 'receipt_seq'),
  cashier_id     uuid NOT NULL REFERENCES users(id),
  subtotal       bigint NOT NULL CHECK (subtotal >= 0),
  discount       bigint NOT NULL DEFAULT 0 CHECK (discount >= 0),
  total          bigint NOT NULL CHECK (total >= 0),
  paid           bigint NOT NULL CHECK (paid >= 0),
  change_amount  bigint NOT NULL DEFAULT 0 CHECK (change_amount >= 0),
  payment_method text NOT NULL CHECK (payment_method IN ('cash', 'qris', 'ewallet', 'card')),
  status         text NOT NULL DEFAULT 'completed' CHECK (status IN ('completed', 'void')),
  voided_by      uuid REFERENCES users(id),
  voided_at      timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  void_reason    text,
  down_payment   bigint NOT NULL DEFAULT 0 CHECK (down_payment >= 0),
  shipping_fee   bigint NOT NULL DEFAULT 0 CHECK (shipping_fee >= 0),
  other_fee      bigint NOT NULL DEFAULT 0 CHECK (other_fee >= 0),
  other_fee_note text,
  CONSTRAINT transactions_total_parts CHECK (total = subtotal - discount + shipping_fee + other_fee),
  CONSTRAINT transactions_void_fields CHECK (
    (status = 'void') = (voided_at IS NOT NULL)
    AND (status = 'void') = (voided_by IS NOT NULL)
    AND (status = 'void') = (void_reason IS NOT NULL)
  )
);

CREATE INDEX transactions_created_idx ON transactions (created_at);
CREATE INDEX transactions_cashier_idx ON transactions (cashier_id, created_at);

-- Nama dan harga produk disalin saat transaksi, supaya struk lama tidak berubah ketika produknya diubah. Isi struk
-- tidak ikut terhapus diam-diam: menghapus struknya ditolak.
CREATE TABLE transaction_items (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  transaction_id uuid NOT NULL REFERENCES transactions(id) ON DELETE RESTRICT,
  product_id     uuid NOT NULL REFERENCES products(id),
  product_name   text NOT NULL,
  unit_price     bigint NOT NULL CHECK (unit_price >= 0),
  qty            integer NOT NULL CHECK (qty > 0),
  discount       bigint NOT NULL DEFAULT 0 CHECK (discount >= 0),
  line_total     bigint NOT NULL CHECK (line_total >= 0)
);

-- Laporan membaca barang terjual per hari dari indeks ini saja, tanpa membuka tabelnya, jadi laporan 7 hari atau
-- sebulan tetap cepat walau datanya sudah bertahun-tahun.
CREATE INDEX transaction_items_tx_cover_idx ON transaction_items (transaction_id) INCLUDE (qty);
CREATE INDEX transaction_items_product_idx ON transaction_items (product_id);

-- ---------- Pre-order ----------

-- total = subtotal (harga barang) + shipping_fee (ongkir) + other_fee (biaya lain, mis. kresek), yaitu yang harus
-- dibayar pelanggan; DP dan sisa tagihan dihitung dari total ini. Pre-order selesai selalu punya struk
-- penyerahannya (transaction_id), dan hanya pre-order selesai yang punya struk. version: lihat bump_version.
CREATE TABLE preorders (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_no       text NOT NULL UNIQUE DEFAULT next_doc_no('PO', 'preorder_seq'),
  customer_name  text NOT NULL,
  customer_phone text,
  due_date       date NOT NULL,                       -- hari H
  due_time       time,
  fulfillment    text NOT NULL CHECK (fulfillment IN ('pickup', 'delivery')),
  address        text,
  notes          text,
  status         text NOT NULL DEFAULT 'ordered' CHECK (status IN ('ordered', 'ready', 'completed', 'cancelled')),
  total          bigint NOT NULL CHECK (total >= 0),
  created_by     uuid NOT NULL REFERENCES users(id),
  transaction_id uuid REFERENCES transactions(id),
  cancelled_by   uuid REFERENCES users(id),
  cancelled_at   timestamptz,
  cancel_reason  text,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  subtotal       bigint NOT NULL CHECK (subtotal >= 0),
  shipping_fee   bigint NOT NULL DEFAULT 0 CHECK (shipping_fee >= 0),
  other_fee      bigint NOT NULL DEFAULT 0 CHECK (other_fee >= 0),
  other_fee_note text,
  version        integer NOT NULL DEFAULT 1,
  CONSTRAINT preorders_delivery_address CHECK (fulfillment <> 'delivery' OR address IS NOT NULL),
  CONSTRAINT preorders_total_parts CHECK (total = subtotal + shipping_fee + other_fee),
  CONSTRAINT preorders_completed_transaction CHECK ((status = 'completed') = (transaction_id IS NOT NULL))
);

CREATE INDEX preorders_open_due_idx ON preorders (due_date) WHERE status IN ('ordered', 'ready');
-- Untuk menjumlah pcs per hari H saat mengecek kuota.
CREATE INDEX preorders_quota_due_idx ON preorders (due_date) WHERE status <> 'cancelled';
CREATE INDEX preorders_created_idx ON preorders (created_at);
-- Satu transaksi paling banyak menyelesaikan satu pre-order. Indeks ini juga dipakai Riwayat untuk menandai
-- transaksi yang berasal dari pre-order (LEFT JOIN preorders lewat transaction_id).
CREATE UNIQUE INDEX preorders_transaction_idx ON preorders (transaction_id) WHERE transaction_id IS NOT NULL;

CREATE TRIGGER trg_preorders_updated BEFORE UPDATE ON preorders
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_preorders_version BEFORE UPDATE ON preorders
  FOR EACH ROW EXECUTE FUNCTION bump_version();

-- Harga dikunci saat pre-order dibuat. Barang dan DP tidak ikut terhapus diam-diam: menghapus pre-order-nya ditolak.
CREATE TABLE preorder_items (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  preorder_id  uuid NOT NULL REFERENCES preorders(id) ON DELETE RESTRICT,
  product_id   uuid NOT NULL REFERENCES products(id),
  product_name text NOT NULL,
  unit_price   bigint NOT NULL CHECK (unit_price >= 0),
  qty          integer NOT NULL CHECK (qty > 0),
  line_total   bigint NOT NULL CHECK (line_total >= 0)
);

CREATE INDEX preorder_items_preorder_idx ON preorder_items (preorder_id);

-- DP (uang muka). Bisa dicatat lebih dari sekali.
CREATE TABLE preorder_payments (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  preorder_id uuid NOT NULL REFERENCES preorders(id) ON DELETE RESTRICT,
  amount      bigint NOT NULL CHECK (amount > 0),
  method      text NOT NULL CHECK (method IN ('cash', 'qris', 'ewallet', 'card')),
  created_by  uuid NOT NULL REFERENCES users(id),
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX preorder_payments_preorder_idx ON preorder_payments (preorder_id);
CREATE INDEX preorder_payments_created_idx ON preorder_payments (created_at);

-- Kuota pre-order per hari: total pcs semua produk PO dengan hari H (due_date) yang sama, dari semua pesanan kecuali
-- yang dibatalkan. Satu baris saja (id = 1), diubah admin dari aplikasi.
CREATE TABLE preorder_settings (
  id          smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  daily_limit integer NOT NULL DEFAULT 5000 CHECK (daily_limit BETWEEN 1 AND 1000000),
  updated_by  uuid REFERENCES users(id),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

INSERT INTO preorder_settings (id) VALUES (1);

-- ---------- Pengiriman ----------

-- Pengiriman dibuat otomatis bersama pre-order "diantar" dan selesai bersama pre-order-nya. transaction_id dan fee
-- hanya terisi di pengiriman lama dari penjualan toko. version: lihat bump_version.
-- Pemesan (customer_*), alamat, jadwal, dan catatan selalu sama dengan pre-order-nya: aplikasi mengubah keduanya
-- bersama. received_by: nama orang yang menerima barangnya, wajib diisi saat ditandai terkirim (kosong di pengiriman
-- yang terkirim sebelum kolom ini ada).
CREATE TABLE deliveries (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  delivery_no     text NOT NULL UNIQUE DEFAULT next_doc_no('PG', 'delivery_seq'),
  preorder_id     uuid REFERENCES preorders(id),
  transaction_id  uuid REFERENCES transactions(id),
  customer_name   text NOT NULL,
  customer_phone  text,
  address         text NOT NULL,
  scheduled_date  date NOT NULL,
  scheduled_time  time,
  courier         text,
  fee             bigint NOT NULL DEFAULT 0 CHECK (fee >= 0),
  notes           text,
  status          text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'on_the_way', 'delivered', 'failed', 'cancelled')),
  status_note     text,                               -- alasan gagal atau batal
  dispatched_at   timestamptz,
  delivered_at    timestamptz,
  created_by      uuid NOT NULL REFERENCES users(id),
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  version         integer NOT NULL DEFAULT 1,
  received_by     text,
  CONSTRAINT deliveries_one_source CHECK ((preorder_id IS NOT NULL) <> (transaction_id IS NOT NULL)),
  CONSTRAINT deliveries_received_by CHECK (received_by IS NULL OR status = 'delivered')
);

CREATE UNIQUE INDEX deliveries_preorder_idx ON deliveries (preorder_id) WHERE preorder_id IS NOT NULL;
CREATE UNIQUE INDEX deliveries_transaction_active_idx ON deliveries (transaction_id)
  WHERE transaction_id IS NOT NULL AND status <> 'cancelled';
CREATE INDEX deliveries_active_due_idx ON deliveries (scheduled_date) WHERE status IN ('pending', 'on_the_way');
CREATE INDEX deliveries_updated_idx ON deliveries (updated_at);

CREATE TRIGGER trg_deliveries_updated BEFORE UPDATE ON deliveries
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_deliveries_version BEFORE UPDATE ON deliveries
  FOR EACH ROW EXECUTE FUNCTION bump_version();

-- ---------- Kunci idempotensi dan riwayat perubahan ----------

-- Kunci request dari client (header Idempotency-Key). Penjualan, pre-order, atau DP yang dikirim ulang, misalnya
-- karena koneksi putus setelah data tersimpan, mendapat hasil yang pertama dan tidak tercatat dua kali. Baris ini
-- ditulis dalam transaksi database yang sama dengan datanya. Server menghapus kunci yang lebih tua dari 30 hari.
CREATE TABLE idempotency_keys (
  request_key  uuid PRIMARY KEY,
  user_id      uuid NOT NULL REFERENCES users(id),
  action       text NOT NULL,
  request_hash text NOT NULL,
  target_id    uuid,
  created_at   timestamptz NOT NULL DEFAULT now()
);

-- Riwayat perubahan data yang menimpa nilai lama: produk (harga, stok, hapus), pre-order (jadwal, biaya, status),
-- pengiriman, akun pengguna, kuota pre-order, dan pembatalan transaksi. changes = {"kolom": [lama, baru]}.
-- entity_label menyimpan nama atau nomor saat kejadian, supaya riwayat tetap terbaca walau datanya diganti.
CREATE TABLE audit_logs (
  id           bigserial PRIMARY KEY,
  created_at   timestamptz NOT NULL DEFAULT now(),
  actor_id     uuid REFERENCES users(id),
  action       text NOT NULL,
  entity       text NOT NULL,
  entity_id    text NOT NULL,
  entity_label text NOT NULL,
  changes      jsonb NOT NULL DEFAULT '{}'::jsonb,
  note         text
);

CREATE INDEX audit_logs_entity_idx ON audit_logs (entity, entity_id, created_at DESC);
CREATE INDEX audit_logs_created_idx ON audit_logs (created_at DESC);
CREATE INDEX audit_logs_actor_idx ON audit_logs (actor_id, created_at DESC);
