# FitGiriş — Kapsamlı Uygulama Planı (Implementation Plan)

## 1. Proje Özeti ve Hedefler
**FitGiriş**, çok şubeli (5 şube) bir spor salonu zincirinin üyelik paketleri, tahsilat ve otomatik yenileme, turnike geçiş kontrolü ve grup dersi rezervasyonlarını yöneten, yüksek eşzamanlılık (concurrency) ve düşük gecikme (latency) gereksinimlerine sahip kurumsal bir sistemdir.

Bu plan; projenin `proje.md` şartnamesine ve `.agents/skills/fitgiris-rules/SKILL.md` kurallarına %100 uyumlu, Docker tabanlı, Go backend + React frontend + PostgreSQL veritabanı + Go Simülatör mimarisinde adım adım geliştirilmesini hedefler.

---

## 2. Mimari ve Teknoloji Yığını

- **Backend:** Go 1.22+ (`chi` yönlendirici), temiz katmanlı mimari (`domain`, `repository`, `service`, `handler`).
- **Veritabanı:** PostgreSQL 16. ORM kesinlikle YASAK; `jackc/pgx/v5` (`pgxpool`) ve `migrations/` altında ham SQL migrasyonları.
- **Para Birimi:** Kesinlikle `BIGINT` (kuruş). Asla `float` kullanılmayacak.
- **Şema Sabitleme:** Struct alanları ve SQL sorguları `migrations/` altındaki `.sql` dosyalarına birebir sadık kalacak.
- **Frontend:** React 18 + TypeScript + Vite + Tailwind CSS. 5 rol için özelleştirilmiş gösterge panelleri (Üye, Resepsiyon, Eğitmen, Salon Müdürü, Muhasebe).
- **Simülatör:** Ayrı bir Go servisi olarak 5 şube, 20.000 üye seed verisi ve canlı olay simülasyonu (turnike, ders yarışı, ödeme hatası).
- **Konteyner Mimarisi:** Multi-stage, `USER nonroot`, `healthcheck` ve `service_healthy` bağımlılıkları ile tek komutta (`docker compose up --build`) 5 dakikada ayağa kalkan orkestrasyon.
- **Grounding Kuralı:** Her Go kodu yazıldığında terminalde `go build ./...` ve `go test ./...` çalıştırılacak.

---

## 3. Dört Zorlu Çekirdek Görevin Çözüm Mimarisi

### Görev 1 (T-1): Üyelik Yaşam Döngüsü & Clamped Date Aritmetiği
- **Problem:** Go `time.AddDate(0, 1, 0)` fonksiyonu 31 Ocak'a 1 ay eklendiğinde artık olmayan 31 Şubat yerine 3 Mart (veya artık yıla göre 2 Mart) üretir.
- **Çözüm (Clamped Date Algoritması):**
  Hedef ayın son gününü bulan `daysInMonth(year, month)` fonksiyonu ile taşmayı engelleyen `AddClampedMonths(t time.Time, months int) time.Time` fonksiyonu yazılır. Eğer kaynak gün (31), hedef ayın maksimum gününden (28/29) büyükse, hedef ayın son gününe sabitlenir (clamp).
- **Durum Makinesi (Finite State Machine - FSM):**
  - Durumlar: `PENDING_PAYMENT`, `ACTIVE`, `FROZEN`, `TRANSFERRED`, `SUSPENDED`, `EXPIRED`, `CANCELLED`.
  - İzin Verilen Geçişler:
    - `ACTIVE` -> `FROZEN` (dondurma gün sayısı bitiş tarihine eklenir; yılda maks 30 gün kuralı).
    - `FROZEN` -> `ACTIVE` (erken çözülme durumunda kullanılmayan dondurma hakkı iade edilir, bitiş tarihi revize edilir).
    - `ACTIVE` -> `TRANSFERRED` (orijinal üyelik kapatılır, hedef üyeye kalan gün kadar yeni aktif üyelik açılır).
    - `ACTIVE` -> `SUSPENDED` (tahsilat 7 gün boyunca başarısız olursa).
    - `SUSPENDED` -> `ACTIVE` (başarılı borç ödemesiyle).
  - Geçersiz tüm durum geçişleri `409 Conflict` ve `ERR_INVALID_STATE_TRANSITION` ile reddedilir.
- **Orantılı Yükseltme (Prorated Upgrade) Formülü:**
  $$\text{Kalan Değer} = \left\lfloor \frac{\text{Kalan Gün}}{\text{Toplam Gün}} \times \text{Mevcut Paket Tutarı (kuruş)} \right\rfloor$$
  $$\text{Ödenecek Fark} = \max\left(0, \text{Yeni Paket Tutarı (kuruş)} - \text{Kalan Değer}\right)$$

### Görev 2 (T-2): Grup Dersi Rezervasyonu & Eşzamanlılık Kilidi
- **Problem:** 50 kişinin son 1 boş kontenjana aynı anda saldırdığı yarış durumu (race condition).
- **Çözüm (Pessimistic Row Locking):**
  ```sql
  SELECT id, capacity, booked_count, start_time 
  FROM class_schedules 
  WHERE id = $1 
  FOR UPDATE;
  ```
  - `booked_count < capacity` ise üye `class_bookings` tablosuna `CONFIRMED` olarak eklenir ve `booked_count` 1 artırılır.
  - Kontenjan doluysa üye `class_waitlist` tablosuna FIFO sırasıyla (`created_at`) eklenir.
  - Aynı üyenin çakışan saatteki başka bir derse kaydolması `EXISTS (...)` kontrolüyle engellenir.
- **2 Saat İptal Kuralı & Bekleme Listesi Terfisi (Waitlist Promotion):**
  - `start_time - now >= 2 hours`: Ücretsiz iptal, hak/kredi iadesi.
  - `start_time - now < 2 hours`: Cezalı iptal (`LATE_CANCELLED`), kredi iadesi yapılmaz.
  - İptal gerçekleştiğinde boşalan koltuk için bekleme listesindeki ilk kişi kilitlenir:
  ```sql
  SELECT id, member_id 
  FROM class_waitlist 
  WHERE schedule_id = $1 AND status = 'WAITING' 
  ORDER BY created_at ASC 
  LIMIT 1 
  FOR UPDATE SKIP LOCKED;
  ```
  İlk bekleyen kişi otomatik olarak `CONFIRMED` statüsüne yükseltilir.

### Görev 3 (T-3): Turnike Karar Motoru (p95 < 50ms) & Çevrimdışı Mod
- **Düşük Gecikme Stratejisi:**
  - Turnike giriş kararı 4 kontrolü içerir:
    1. Üyelik statüsü `ACTIVE` mi ve `end_date >= today` mi?
    2. Kalan giriş hakkı var mı (`remaining_entries > 0`)?
    3. Şube yetkisi geçerli mi (`all_branches = true` veya `branch_id = current_branch`)?
    4. **Anti-Passback Kontrolü:** Son 2 saat içinde aynı şubede giriş yapılmış ve henüz çıkış yapılmamışsa `ERR_ANTI_PASSBACK` verilir.
  - Hız için Go belleğinde thread-safe `sync.RWMutex` ile korunan `TurnstileGateCache` tutulur. Aktif kart/QR tokenları ve son 2 saatlik geçiş damgaları bellekte tutularak veritabanı disk I/O yükü bertaraf edilir; p95 < 15ms hedeflenir.
  - Veritabanı tarafında hızlı composite indeksler (`idx_access_logs_member_turnstile_time`) tanımlanır.
- **Çevrimdışı (Offline) Mod Mimarisi:**
  - Turnike cihazı yerel SQLite / bellek kuyruğunda son senkronize edilmiş beyaz listeyi tutar.
  - Ağ kesildiğinde yerel beyaz listeye göre kapıyı açar ve geçişleri yerel kuyruğa yazar.
  - Ağ geldiğinde `POST /api/v1/turnstiles/sync-offline` ucuyla geriye dönük loglar gönderilir.
  - Çakışmalar `audit_logs` tablosuna `FLAGGED_OFFLINE_ANOMALY` olarak işlenir ve salon müdürüne bildirim üretilir.

### Görev 4 (T-4): Otomatik Yenileme & Boşluksuz (Gapless) Fatura Numarası
- **Sahte Ödeme Sağlayıcı (Payment Provider Simulator):**
  - %20 oranında rassal geçici banka/ağ hatası simüle eder (`ERR_BANK_TIMEOUT`, `ERR_INSUFFICIENT_FUNDS`).
- **3 Deneme Takvimi ve Askıya Alma (Suspension):**
  - 1. Gün: İlk ödeme denemesi. Başarısız olursa `FAILED_RETRY_1` ve 48 saat sonraya randevu.
  - 3. Gün: 2. ödeme denemesi. Başarısız olursa `FAILED_RETRY_2` ve 48 saat sonraya randevu.
  - 5. Gün: 3. ödeme denemesi. Başarısız olursa `FAILED_RETRY_3`.
  - 7. Gün: Hala tahsil edilemediyse üyelik `SUSPENDED` statüsüne alınır, turnike geçişleri kapatılır.
- **Idempotency-Key Başlığı:**
  - Kritik `POST` isteklerinde (ödeme, fatura, rezervasyon) `Idempotency-Key` zorunlu tutulur. Tekrarlanan isteklerde aynı işlem tekrarlanmaz, önbelleğe alınan yanıt dönülür.
- **Boşluksuz Sıralı Fatura Üretimi (Gapless Invoicing):**
  - PostgreSQL `SEQUENCE` nesneleri `ROLLBACK` durumunda harcanan numarayı geri alamaz ve boşluk (gap) bırakır.
  - Bu nedenle özel `invoice_sequences` sayaç tablosu kullanılır:
  ```sql
  SELECT current_val 
  FROM invoice_sequences 
  WHERE year = $1 AND branch_id = $2 
  FOR UPDATE;
  ```
  - Sayaç 1 artırılır, fatura `FIT-2026-IST01-000001` formatında üretilir. Transaction iptal olursa kilit serbest kalır ve sayaç artmaz. Böylece %100 boşluksuz ve mükerrersiz seri no elde edilir.

---

## 4. Fazlara Ayrılmış Uygulama Yol Haritası ve Durum Tablosu

| Faz | Başlık | Kapsam | Durum |
| :--- | :--- | :--- | :--- |
| **Faz 1** | **Altyapı, Konteynerler ve DB Şeması** | Go modülü, Dockerfile'lar, Compose, PostgreSQL 16 tabloları, `migrations/`, Healthcheck | ✅ Tamamlandı |
| **Faz 2** | **Auth, RBAC & Temel Middleware** | Argon2id/Bcrypt, JWT, RBAC (5 rol), Audit Logs, Idempotency, Keyset sayfalama | ✅ Tamamlandı |
| **Faz 3** | **Görev 1 (T-1) & F-1: Üyelik & Paketler** | Date clamping, FSM durum makinesi, dondurma, devir, orantılı yükseltme, paket satışı | ✅ Tamamlandı |
| **Faz 4** | **Görev 2 (T-2) & F-4: Grup Dersi & Eşzamanlılık** | Ders programı, `FOR UPDATE` kontenjan kilidi, 2 saat iptal kuralı, `SKIP LOCKED` waitlist | ✅ Tamamlandı |
| **Faz 5** | **Görev 3 & 4 (T-3, T-4): Turnike Kararı, Anti-Passback, Boşluksuz Fatura & Tahsilat** | In-memory gate cache (p95 < 50ms), 2h anti-passback, offline sync, FOR UPDATE fatura sayacı, 3 retry & askıya alma, idempotent ödeme | ✅ Tamamlandı |
| **Faz 6** | **Simülatör, Otomasyon & Doğrulama (F-5, F-6, Seed, Verifier)** | PT paket & randevuları, Ciro/Doluluk/Eğitmen raporları, Go Simülatörü, Seed=42 (20.000 üye), `make verify`, `make reset-demo`, `TESTING.md`, k6 yük testi | ✅ Tamamlandı |
| **Faz 7** | **React + TypeScript + Vite Frontend Portalları** | 5 rol portalı (Üye QR/bakiye, Resepsiyon turnike, Eğitmen yoklama/PT, Müdür, Muhasebe) | ⏳ Sırada |
| **Faz 8** | **Son Teslim Hijyeni, OpenAPI, DECISIONS.md & README Soruları** | OpenAPI (Swagger), DECISIONS.md (3 ADR), README doğrulama soruları, GitHub teslim hazırlığı | ⏸️ Beklemede |

---

## 5. Tamamlanan Faz 6 Detayları (Simülatör, Otomasyon, PT, Raporlar ve Doğrulama)
1. Kişisel Antrenman (PT) modülü (F-5) tamamlandı: PT paket alımı, randevu takvimi çakışma kontrolü (`HasTrainerConflictTx`) ve iptalde seans iadesi.
2. Finansal Ciro Raporu, 24 saatlik Doluluk Yoğunluk Grafiği ve Eğitmen Performans Raporu (F-6) tamamlandı.
3. Deterministik Tohumlayıcı (Seed=42): 5 şube, 5 demo hesabı, standart paketler, turnikeler ve `pgx.CopyFrom` ile ~1.5 saniyede veritabanına yazılan 20.000 üye geliştirildi.
4. Go Simülatör servisi (`simulator/cmd/simulator`): Docker compose servisi olarak turnike geçişleri, ders rezervasyon hücumları ve otomatik yenileme sağlayıcı hata denemelerini periyodik olarak canlı simüle ediyor.
5. Tek komutla uçtan uca doğrulama paketi (`make verify` / `cmd/verify`): 13 doğrulama kimliğini (`HEALTH`, `AUTH`, `RBAC`, `F-1`..`F-6`, `T-1`..`T-4`) çalıştırıp ASCII PASS/FAIL tablosu basıyor ve 0/1 çıkış kodu üretiyor.
6. `make reset-demo`: Tek komutla veritabanını sıfırlayıp ilk deterministik seed haline döndürüyor.
7. `make load` ve `scripts/turnstile_load_test.js`: Turnike karar hızını 1.000 eşzamanlı istekle test ederek p95 < 5ms (< 50ms) kanıtlıyor.
8. `TESTING.md` ve `requests.http`: Demo hesapları, manuel arayüz senaryoları ve tam API koleksiyonu hazırlandı.
9. `docs/phase-6-summary.md` dokümante edildi.

---

## 5. Tamamlanan Faz 5 Detayları (Turnike, Anti-Passback, Boşluksuz Fatura & Tahsilat)
1. Turnike karar motoru ve in-memory TurnstileGateCache ile p95 gecikmesi 518 mikrosaniyeye (~0.5ms) indirildi.
2. 2 saatlik Anti-Passback kuralı kodlandı (çıkış yapılmadan 2 saat içinde mükerrer giriş engellenir).
3. Çevrimdışı turnike loglarının senkronizasyonu (POST /turnstiles/sync-offline) ve anomali tespiti eklendi.
4. invoice_sequences üzerinde SELECT ... FOR UPDATE ile eşzamanlı 50 istek altında dahi boşluksuz (gapless) ve mükerrersiz fatura üretimi kanıtlandı.
5. Idempotent ödeme akışı ve Idempotency-Key desteği entegre edildi.
6. 3 deneme takvimi (1., 3., 5. gün) ve 7. günde tahsil edilemeyen üyelikleri SUSPENDED durumuna alarak turnikeleri kilitleyen otomatik yenileme kuyruğu geliştirildi.
7. Tüm birim, entegrasyon ve concurrency testleri başarıyla geçti (docs/phase-5-summary.md oluşturuldu).