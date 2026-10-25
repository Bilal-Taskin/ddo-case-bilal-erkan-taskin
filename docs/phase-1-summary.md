# Faz 1 Özet Raporu: Altyapı, Konteynerler ve Veritabanı Şeması

## 1. Genel Bakış
Faz 1 kapsamında FitGiriş spor salonu yönetim sisteminin temel mimari altyapısı, Docker konteyner orkestrasyonu, PostgreSQL 16 veritabanı şeması ve DDL migrasyon sistemi kurulmuştur.

## 2. Yapılan Teknik Geliştirmeler
- **Go Modülü ve Dizin Yapısı:** `fitgiris` modülü Go 1.22+ ve temiz katmanlı mimari (`cmd`, `internal/domain`, `internal/repository`, `internal/service`, `internal/handler`, `internal/platform`, `migrations`) ile yapılandırıldı.
- **Ham SQL Migrasyonları (`jackc/pgx/v5`):** ORM kullanımı yasaklanarak `pgxpool.Pool` üzerinden çalışan gömülü (`embed.FS`) ve otomatik çalışan SQL migrasyon mekanizması geliştirildi:
  - `001_branches_and_users.sql`: Şube ve 5 rolü destekleyen kullanıcı tabloları.
  - `002_memberships_and_packages.sql`: Paket tipleri, üye üyelikleri, dondurma ve devir tabloları.
  - `003_classes_and_bookings.sql`: Grup dersleri, eğitmenler, program, rezervasyon ve bekleme listesi tabloları.
  - `004_turnstile_and_access.sql`: Turnikeler, geçiş logları, QR/kart kimlikleri ve çevrimdışı senkronizasyon tablosu.
  - `005_billing_and_invoices.sql`: Boşluksuz fatura sayaçları, faturalar, ödemeler ve otomatik yenileme kuyruğu.
  - `006_pt_and_audits.sql`: Bireysel antrenman (PT) paket/randevuları, denetim kaydı (`audit_logs`) ve `idempotency_keys` tabloları.
  - `007_refresh_tokens.sql`: Güvenli JWT oturum yenileme ve token rotation tablosu.
- **Konteyner Mimarisi:** Multi-stage, `USER nonroot`, `healthcheck` ve `service_healthy` bağımlılıklarıyla Docker Compose ortamı hazırlandı.
- **Sağlık Kontrolü:** `GET /health` ucu ile DB bağlantı durumu raporlandı.

## 3. Mimari Kararlar
- **ORM Yasağı:** Yüksek eşzamanlılık ve satır kilitleri (`SELECT FOR UPDATE`) üzerinde tam kontrol için doğrudan `jackc/pgx/v5` tercih edildi.
- **Kuruş Tabanlı Para Formatı:** Kayan noktalı sayı (`float`) yuvarlama hatalarını önlemek için tüm parasal alanlar `BIGINT` (kuruş) olarak sabitlendi.
