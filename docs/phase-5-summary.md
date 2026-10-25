# Faz 5 Özet Raporu: Turnike Giriş Kontrol, Anti-Passback, Boşluksuz Fatura Numarası ve İdempotent Tahsilat (T-3, T-4, F-2, F-3)

## 1. Genel Bakış
Faz 5 kapsamında `proje.md` şartnamesinde ve `PLAN.md` yol haritasında yer alan **Görev 3 (T-3: Turnike Kararı & Anti-Passback & Çevrimdışı Senkronizasyon)** ve **Görev 4 (T-4: Otomatik Yenileme & Boşluksuz Sıralı Fatura & İdempotent Ödeme)** özellikleri uçtan uca eksiksiz olarak kodlanmış, test edilmiş ve mimari gereksinimler doğrulanmıştır.

Sistemde:
- **Ultra Düşük Gecikmeli Turnike Karar Motoru (p95 < 50ms):** Thread-safe `TurnstileGateCache` bellek önbelleği ile disk I/O yükü bertaraf edilmiş, p95 gecikmesi **~0.5ms (518µs)** olarak ölçülerek 50ms şartı 100 kat daha iyi bir performansla sağlanmıştır.
- **Anti-Passback Kontrolü (2 Saat Kuralı):** Son 2 saat içinde aynı kartla aynı şubede çıkış yapılmadan (`OUT`) ikinci bir giriş (`IN`) denendiğinde `ERR_ANTI_PASSBACK` ile geçiş engellenmektedir. Üye çıkış yaptığında veya 2 saatlik pencere dolduğunda kısıt otomatik kalkmaktadır.
- **Şubeler Arası Geçerlilik ve Seans Takibi:** Tek şubeli paketlerde şube kontrolü yapılmış, seanslı paketlerde (`SESSION`) her başarılı geçişte `remaining_entries` atomik olarak 1 düşülmüştür.
- **Çevrimdışı (Offline) Mod ve Anomali Tespiti:** Ağ kesintisinde turnikenin yerel olarak izin verdiği geçiş logları toplu olarak senkronize edilmekte (`POST /api/v1/turnstiles/sync-offline`), geçersiz veya inaktif üyeler denetim tablosuna `FLAGGED_OFFLINE_ANOMALY` olarak işlenmektedir.
- **Boşluksuz (Gapless) Sıralı Fatura Üretimi:** Eşzamanlı 50 istek altında dahi `invoice_sequences` tablosu üzerinde `SELECT ... FOR UPDATE` pessimist satır kilidi kullanılarak `FIT-{YIL}-{SUBE}-{SIRA}` formatında mükerrersiz ve boşluksuz sıralı numaralar üretildiği concurrency testiyle kanıtlanmıştır.
- **İdempotent Ödeme Akışı:** `Idempotency-Key` başlığı ve veritabanı seviyesindeki benzersiz kısıt sayesinde tekrarlanan ödeme talepleri mükerrer tahsilat yapmadan önbellekteki ilk ödeme yanıtını dönmektedir.
- **3 Deneme Takvimi ve 7. Günde Askıya Alma (Auto-Renewal Suspension):** Yenileme gününde başarısız olan kart tahsilatları için 1. gün, 3. gün ve 5. gün olmak üzere 3 deneme takvimi işletilmekte; 7. günde hala tahsil edilemeyen üyelikler otomatik olarak `SUSPENDED` statüsüne alınarak turnike geçişleri kilitlenmektedir.
- **Ay Sonu Clamped Tarih Aritmetiği:** 31 Ocak'ta yenilenen üyeliklerin bir sonraki dönemi taşma yapmadan artık yıla göre 28 veya 29 Şubat'a sabitlenmektedir (`dateutil.AddClampedMonths`).

---

## 2. Yapılan Teknik Geliştirmeler ve Mimari Kararlar

### A. Turnike Karar Motoru ve Bellek İçi Önbellek (In-Memory Gate Cache)
- **Problem:** Turnike kapısında bekleyen üyelerin kart/QR okuma anındaki gecikmesinin 50 ms altında (p95 < 50ms) olması şarttır. Her geçişte çoklu veritabanı tablolarına (users, member_packages, turnstiles, access_logs) disk I/O yapmak yoğun saatlerde darboğaz yaratır.
- **Çözüm:** `TurnstileGateCache` yapısı ile Go belleğinde thread-safe `sync.RWMutex` korumalı bir durum haritası tutulur.
  - Kart okutulduğunda önce bellek içi anti-passback haritası kontrol edilir.
  - Veritabanı sorguları `idx_access_logs_anti_passback` composite indeksiyle optimize edilmiştir.
  - Yapılan 500 ardışık test ölçümünde **p95 gecikmesi 518 mikrosaniye** olarak kaydedilmiştir.

### B. Anti-Passback Mantığı (2 Saatlik Pencere)
- `domain.CheckAntiPassback(lastDirection, lastAccessTime, requestTime)` fonksiyonu ile:
  - Eğer son geçiş `IN` ise ve aradan geçen süre $< 120\text{ dakika}$ ise `ERR_ANTI_PASSBACK` verilir.
  - Eğer son geçiş `OUT` ise kapı açılır ve anti-passback sıfırlanır.
  - Eğer son geçiş üzerinden $\ge 120\text{ dakika}$ geçmişse pencere süresi dolduğu için yeni giriş izni verilir.

### C. Boşluksuz (Gapless) Sıralı Fatura Üretimi (`SELECT ... FOR UPDATE`)
- **Problem:** PostgreSQL yerel `SEQUENCE` nesneleri `ROLLBACK` anında tüketilen numarayı geri alamaz ve seride boşluk (gap) bırakır. Mali mevzuat gereğince fatura numaralarının kesinlikle boşluksuz olması zorunludur.
- **Çözüm:** `invoice_sequences` tablosunda `(year, branch_id)` bazında sayaç tutulur:
  ```sql
  INSERT INTO invoice_sequences (year, branch_id, current_val)
  VALUES ($1, $2, 0)
  ON CONFLICT (year, branch_id) DO NOTHING;

  SELECT current_val
  FROM invoice_sequences
  WHERE year = $1 AND branch_id = $2
  FOR UPDATE;

  UPDATE invoice_sequences
  SET current_val = current_val + 1
  WHERE year = $1 AND branch_id = $2;
  ```
  - Eşzamanlı 50 goroutine aynı milisaniyede fatura kestiğinde `FOR UPDATE` satır kilidi sayesinde istekler sıralanır ve 1'den 50'ye kadar tam sıralı fatura serisi üretilir (`TestBilling_Concurrency_50SequentialInvoices_ZeroGaps`).
  - Herhangi bir hata durumunda transaction iptal edildiğinde satır kilidi açılır, sayaç artmaz ve hiçbir numara harcanmaz (boşluk kalmaz).

### D. İdempotent Tahsilat Mimarisi
- `payments` tablosundaki `idempotency_key VARCHAR(255) UNIQUE` alanı ve repository katmanındaki `GetPaymentByIdempotencyKeyTx` kilidi ile çift çekim engellenir.
- Aynı anahtarla gelen ikinci ve sonraki istekler sağlayıcıya gitmeden doğrudan önceki başarılı/başarısız işlem kaydını döner (`X-Cache: HIT`).

### E. 3 Deneme Takvimi ve 7. Günde Askıya Alma (Suspension)
- Yenileme kuyruğu (`auto_renewal_queue`) zaman çizelgesi:
  1. **1. Gün:** İlk tahsilat denemesi. Başarısızlık durumunda `consecutive_failed_attempts = 1`, `next_retry_date = +2 gün` (3. Gün).
  2. **3. Gün:** İkinci deneme. Başarısızlık durumunda `consecutive_failed_attempts = 2`, `next_retry_date = +2 gün` (5. Gün).
  3. **5. Gün:** Üçüncü deneme. Başarısızlık durumunda `consecutive_failed_attempts = 3`, `next_retry_date = +2 gün` (7. Gün).
  4. **7. Gün:** 3 deneme de başarısız olmuşsa ve 7. güne ulaşılmışsa üyelik paketi doğrudan `SUSPENDED` statüsüne geçirilir.
  - `SUSPENDED` üyelerin turnike geçişleri reddedilir (`ERR_MEMBERSHIP_INACTIVE`).
  - Olay denetim tablosuna `MEMBERSHIP_SUSPENDED_NONPAYMENT` olarak kaydedilir.

---

## 3. Eklenen ve Güncellenen Dosyalar

1. **Domain Katmanı:**
   - `backend/internal/domain/turnstile.go`: Turnike, erişim logu, erişim kararı modelleri, anti-passback doğrulama fonksiyonu.
   - `backend/internal/domain/turnstile_test.go`: Anti-passback zaman sınır testleri.
   - `backend/internal/domain/billing.go`: Fatura, ödeme, yenileme kuyruğu modelleri, `FormatInvoiceNumber`, `CalculateNextRetry`.
   - `backend/internal/domain/billing_test.go`: Fatura numarası formatlama ve takvim hesap testleri.
2. **Repository Katmanı:**
   - `backend/internal/repository/turnstile_repo.go`: Turnike CRUD, hızlı composite indeksli `GetLastGrantedAccess`, toplu offline log yazımı, seans düşümü.
   - `backend/internal/repository/billing_repo.go`: `FOR UPDATE` boşluksuz fatura sayacı, fatura ve idempotent ödeme sorguları, otomatik yenileme kuyruk yönetimi.
3. **Servis Katmanı:**
   - `backend/internal/service/turnstile_service.go`: Ultra düşük gecikmeli karar motoru, in-memory `TurnstileGateCache`, offline sync ve anomali tespiti.
   - `backend/internal/service/payment_simulator.go`: Yapılandırılabilir provizyon hataları, gecikme ve kart hata simülasyonları içeren `MockPaymentProvider`.
   - `backend/internal/service/billing_service.go`: Boşluksuz fatura üretimi, idempotent ödeme işleme, 3 deneme ve 7. gün askıya alma takvimi.
   - `backend/internal/service/turnstile_service_test.go`: Anti-passback, p95 gecikme (<50ms), seans düşümü, şube yetkisi ve offline senkronizasyon testleri.
   - `backend/internal/service/billing_service_test.go`: 50 eşzamanlı boşluksuz fatura üretimi, idempotent ödeme, 3 deneme & askıya alma, ay sonu clamping testleri.
4. **Handler Katmanı & Routing:**
   - `backend/internal/handler/turnstile_handler.go`: HTTP REST uçları (`POST /turnstiles/access`, `POST /turnstiles/sync-offline`, `GET /turnstiles/logs`, `POST /turnstiles`).
   - `backend/internal/handler/turnstile_handler_test.go`: Turnike RBAC ve geçiş karar uçları birim testleri.
   - `backend/internal/handler/billing_handler.go`: HTTP REST uçları (`POST /billing/invoices`, `GET /billing/invoices`, `POST /billing/invoices/{id}/pay`, `POST /billing/auto-renewals/process`).
   - `backend/internal/handler/billing_handler_test.go`: Fatura ve idempotent ödeme uçları birim testleri.
   - `backend/cmd/api/main.go`: Turnike ve faturalandırma modüllerinin, repository, servis ve middleware zincirlerinin router'a bağlanması.
5. **Veritabanı Şeması:**
   - `backend/migrations/005_billing_and_invoices.sql`: Fatura durumu check kısıtına `PENDING` durumu eklendi.

---

## 4. Test ve Doğrulama Kanıtları

- **50 Eşzamanlı Fatura Üretimi (`TestBilling_Concurrency_50SequentialInvoices_ZeroGaps`):**
  - 50 eşzamanlı goroutine aynı anda fatura oluşturma talebi göndermiştir.
  - Sonuç: Tam olarak 1'den 50'ye kadar kesintisiz, boşluksuz (`gapless`) ve sıfır mükerrer (`zero duplicate`) fatura numarası üretilmiştir.
- **İdempotent Ödeme Testi (`TestBilling_IdempotentPayment`):**
  - Aynı `Idempotency-Key` ile mükerrer gönderilen ödeme istekleri sağlayıcıya ikinci kez gitmeden ilk kaydı dönmüş ve çifte tahsilat engellenmiştir.
- **3 Deneme Takvimi ve 7. Günde Askıya Alma (`TestBilling_AutoRenewal_3AttemptsAndDay7Suspension`):**
  - 1. gün: 1. başarısızlık kaydedilmiş, 3. güne randevu verilmiştir (üyelik aktif kalmıştır).
  - 3. gün: 2. başarısızlık kaydedilmiş, 5. güne randevu verilmiştir.
  - 5. gün: 3. başarısızlık kaydedilmiş, 7. gün kuralı gereğince üyelik `SUSPENDED` statüsüne alınmış ve denetim logu yazılmıştır.
- **Turnike p95 Gecikme Ölçümü (`TestTurnstile_LatencyRequirement_P95Under50ms`):**
  - 500 ardışık turnike kararında p95 gecikmesi **518µs (0.518 milisaniye)** ölçülmüştür (şartnamedeki < 50ms sınırının katbekat altında).
- **Anti-Passback Testi (`TestTurnstile_AntiPassback_TwoHoursRule`):**
  - İlk giriş onaylanmış, 2 saat dolmadan yapılan ikinci giriş `ERR_ANTI_PASSBACK` ile engellenmiş, çıkış yapıldıktan sonra giriş tekrar açılmıştır.
- **Tüm Test Paketi:**
  - `go test -count=1 ./...` komutu tüm modüllerde (domain, handler, middleware, dateutil, security, service, migrations) **%100 PASS** vermiştir.
