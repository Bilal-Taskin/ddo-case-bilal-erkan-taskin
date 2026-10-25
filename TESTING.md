# FitGiriş — Test, Doğrulama ve Elle Deneme Kılavuzu (TESTING.md)

Bu kılavuz, **DDO Yazılım Ekibi Proje Şartnamesi (`proje.md`) Bölüm 10** gereğince hazırlanmıştır. Projedeki tüm işlevsel modüllerin (`F-1` .. `F-6`), dört zorlu çekirdek görevin (`T-1` .. `T-4`) ve sistem güvenlik kontrollerinin tek komutla veya adım adım elle nasıl doğrulanacağını ayrıntılı olarak açıklamaktadır.

---

## 1. Demo Kullanıcı Hesapları

Tüm roller için başlangıç parolası ortaktır: **`Password123!`**

| Rol | E-Posta | Parola | Yetki Alanı / Sorumluluk |
| :--- | :--- | :--- | :--- |
| **Salon Müdürü (MANAGER)** | `manager@fitgiris.com` | `Password123!` | Tüm şubeler, paket tanımlama, eğitmen/ciro raporları, sınıf oluşturma |
| **Muhasebe (ACCOUNTANT)** | `accountant@fitgiris.com` | `Password123!` | Faturalama, tahsilat, otomatik yenileme kuyruğu, mali ciro analitiği |
| **Resepsiyon (RECEPTION)** | `reception@fitgiris.com` | `Password123!` | Turnike geçiş kontrolü, manuel giriş/çıkış, çevrimdışı log senkronizasyonu |
| **Eğitmen (TRAINER)** | `trainer@fitgiris.com` | `Password123!` | Grup dersi programı, yoklama tamamlama, PT seans randevuları |
| **Üye (MEMBER)** | `member@fitgiris.com` | `Password123!` | Paket satın alma, dondurma, transfer, yükseltme, ders ve PT randevusu |

> Ek olarak veri tohumlayıcı (seed) tarafından `member_00001@fitgiris.com` ile `member_20000@fitgiris.com` arasında 20.000 aktif/süresi dolmuş üye hesabı açılmıştır.

---

## 2. Hızlı Doğrulama ve Otomasyon Komutları

### A. Tek Komutla Uçtan Uca Doğrulama (`make verify`)
Sistem ayağa kalktıktan sonra aşağıdaki komut çalıştırıldığında tüm kimlikler (`HEALTH`, `AUTH`, `RBAC`, `F-1`..`F-6`, `T-1`..`T-4`) denenir ve sonuç tablosu basılır:

```bash
make verify
# Veya Docker içinde çalıştırmak için:
docker compose --profile verify run --rm verify
# Veya doğrudan Go ile:
cd backend && go run ./cmd/verify
```

### B. Demo Verilerini İlk Haline Sıfırlama (`make reset-demo`)
Veritabanını sıfırlamak ve Seed=42 tohumuyla temiz 5 şube, paketler, turnikeler ve 20.000 üyeyi yeniden yüklemek için:

```bash
make reset-demo
# Veya doğrudan Go ile:
cd backend && go run ./cmd/seed --reset
```

### C. Turnike Karar Motoru Yük Testi (`make load`)
Görev 3 kapsamındaki p95 < 50ms turnike karar hızını 1.000 eşzamanlı istekle ölçmek için:

```bash
make load
# Veya k6 ile çalıştırmak için:
k6 run scripts/turnstile_load_test.js
```

---

## 3. Dört Zorlu Görev İçin Kanıt Komutları (Proof Commands)

| Görev Kimliği | Görev Tanımı | Kanıt Komutu | Başarı Kriteri |
| :--- | :--- | :--- | :--- |
| **`T-1`** | **Üyelik Yaşam Döngüsü ve Clamped Date** | `cd backend && go test -v ./internal/service -run TestMembershipLifecycle` | 31 Ocak + 1 ay artık yılda 29 Şubat, normal yılda 28 Şubat; FSM durum geçişleri; kalan değer hesabı |
| **`T-2`** | **Grup Dersi Rezervasyonu (50 Kişi Yarışı & Waitlist)** | `cd backend && go test -v ./internal/service -run TestBooking_Concurrency_50Members1Slot` | 50 eşzamanlı istekte tam 1 kişi CONFIRMED, 49 kişi WAITING; 2 saat iptalinde FIFO terfi |
| **`T-3`** | **Turnike Kararı (p95 < 50ms) & Anti-Passback** | `cd backend && go test -v ./internal/service -run TestTurnstileService_LatencyBenchmark` veya `make load` | In-memory cache ile p95 < 5ms (< 50ms kuralı); 2 saat içinde mükerrer girişe `ERR_ANTI_PASSBACK` |
| **`T-4`** | **Otomatik Yenileme & Boşluksuz Fatura Sırası** | `cd backend && go test -v ./internal/service -run TestBilling_Concurrency_50SequentialInvoices_ZeroGaps` | `SELECT ... FOR UPDATE` ile 50 eşzamanlı faturada 1..50 tam ardışık ve sıfır boşluk; 3 deneme takvimi |

---

## 4. Doğrulama Kimlikleri Matrisi: Nasıl Denenir ve Beklenen Sonuç

| Kimlik | Kapsam / Görev | Nasıl Denenir (API / cURL / HTTP) | Beklenen Sonuç |
| :--- | :--- | :--- | :--- |
| **`HEALTH`** | Sistem Sağlığı | `GET /health` | HTTP 200 `{"status":"healthy","database":"connected"}` |
| **`AUTH`** | Kimlik Doğrulama | `POST /api/v1/auth/login` (Hatalı parola: `YanlisParola`) | HTTP 401 Unauthorized `ERR_INVALID_CREDENTIALS` |
| **`RBAC`** | Yetki İhlali Engeli | `GET /api/v1/manager/dashboard` (Üye JWT tokenı ile) | HTTP 403 Forbidden `FORBIDDEN` |
| **`F-1`** | Üyelik Paketleri & Satış | `POST /api/v1/memberships/purchase` (`package_type_id`, `card_token`) | HTTP 201 Created, `status: ACTIVE`, bitiş tarihi clamped |
| **`F-2`** | Fatura & İdempotent Ödeme | `POST /api/v1/billing/invoices/{id}/pay` (`Idempotency-Key: X`) 2 kez gönder | İlk istekte tahsilat HTTP 200; aynı anahtarla ikinci istekte mükerrer çekim yapılmadan önceki yanıt dönülür |
| **`F-3`** | Turnike Giriş Kontrol | `POST /api/v1/turnstiles/access` (`card_token: CARD-DEMO-001`) | HTTP 200 `status: GRANTED`, seanslı pakette `remaining_entries` -1 |
| **`F-4`** | Grup Dersi Rezervasyon | `POST /api/v1/classes/schedules/{id}/book` (Üye oturumu ile) | Boş yer varsa `CONFIRMED`; kontenjan doluysa FIFO `WAITING` |
| **`F-5`** | Kişisel Antrenman (PT) | `POST /api/v1/pt/packages` ve `POST /api/v1/pt/appointments` | Randevu çakışması yoksa `CONFIRMED`; eğitmen meşgulse `ERR_PT_TRAINER_CONFLICT` |
| **`F-6`** | Mali & Operasyon Raporları | `GET /api/v1/reports/revenue`, `/reports/occupancy`, `/reports/trainers` | Şube bazlı ciro toplamları, 24 saatlik giriş/çıkış grafiği, eğitmen doluluk yüzdeleri |
| **`T-1`** | Clamped Date & FSM | `POST /api/v1/memberships/{id}/freeze` ardından `unfreeze` | Dondurulan gün kadar `end_date` ötelenir; geçersiz geçişte HTTP 409 Conflict |
| **`T-2`** | Eşzamanlı Rezervasyon | Tek kontenjanlı derse 50 goroutine ile eşzamanlı POST isteği | Yalnızca 1 goroutine başarılı, 49'u bekleme listesine alınır |
| **`T-3`** | Turnike Kararı & Anti-Pass | `POST /api/v1/turnstiles/access` ardı ardına 2 kez giriş yönünde | 1. istek `GRANTED`; çıkış yapılmadan 2 saat içindeki 2. istek `ERR_ANTI_PASSBACK` ile `DENIED` |
| **`T-4`** | Boşluksuz Sıralı Fatura | 50 goroutine ile aynı anda fatura kesme isteği | Fatura numaraları `FIT-2026-IST01-000001` .. `FIT-2026-IST01-000050` arasında atlamasız üretilir |

---

## 5. Arayüzde (Frontend) Elle Denenecek Kısa Senaryolar

Arayüz `http://localhost:3000` adresinde açıldığında aşağıdaki senaryolar adım adım izlenebilir:

### Senaryo 1: Üye Portalı — QR Kod ve Ders Rezervasyonu
1. `member@fitgiris.com` / `Password123!` ile giriş yapın.
2. Dashboard'da üyenin aktif paketi, kalan gün sayısı ve turnike geçiş QR kodu görüntülenir.
3. Sol menüden **"Grup Dersleri"** sekmesine geçin; bugünkü Spinning dersine **"Rezerve Et"** butonuna tıklayın.
4. Başarılı bildirimini ve rezerve edilen dersler listesinde statünün `CONFIRMED` olduğunu görün.

### Senaryo 2: Resepsiyon — Canlı Turnike Ekranı ve Anti-Passback İhlali
1. `reception@fitgiris.com` / `Password123!` ile giriş yapın.
2. Turnike kontrol panelinde kart okutma kutusuna `CARD-DEMO-001` yazıp **"Giriş Yap (IN)"** butonuna basın.
3. Kapı yeşil yanar (`GRANTED`) ve geçiş anlık log tablosuna düşer (Gecikme < 10ms).
4. Çıkış yapmadan aynı karta hemen tekrar **"Giriş Yap (IN)"** butonuna basın.
5. Sistem anında kırmızı uyarı verir: **"İhlal: Anti-Passback Kuralı (2 saat dolmadan mükerrer giriş)"**.
6. **"Çıkış Yap (OUT)"** butonuna basıldığında turnike açılır ve kısıtlama sıfırlanır.

### Senaryo 3: Eğitmen — Yoklama ve Kişisel Antrenman (PT)
1. `trainer@fitgiris.com` / `Password123!` ile giriş yapın.
2. Bugünkü ders programında yer alan derse tıklayıp kayıtlı üyeleri listeleyin.
3. Üyenin yanındaki **"Geldi (Attended)"** butonuna basarak yoklamayı tamamlayın.
4. **"PT Randevuları"** sekmesinden Bilal Üye ile olan randevuyu görüntüleyin ve **"Tamamlandı"** olarak işaretleyin.

### Senaryo 4: Salon Müdürü ve Muhasebe — Ciro ve Yoğunluk Raporu
1. `manager@fitgiris.com` veya `accountant@fitgiris.com` ile giriş yapın.
2. **"Finansal Raporlar"** ekranında Beşiktaş, Kadıköy, Maslak, Çankaya ve Alsancak şubelerinin ciro dağılımını inceleyin.
3. **"Doluluk Yoğunluk Grafiği"** üzerinde saat bazlı (08:00 - 22:00) pik saatleri (ör. 19:00 akşam yoğunluğu) inceleyin.
4. Sıralı fatura listesinde fatura numaralarının (`FIT-2026-IST01-...`) hiçbir atlama olmadan sıralandığını doğrulayın.

---

## 6. Elle Deneme HTTP Koleksiyonu (`requests.http`)

VS Code içerisinde `REST Client` (Humao) eklentisi yüklüyse, repo kökündeki [`requests.http`](file:///C:/Users/MONSTER/OneDrive/Desktop/gym_center_project/requests.http) dosyası açılarak her isteğin üzerindeki **"Send Request"** butonuna tıklanarak tüm akış canlı denenebilir.
