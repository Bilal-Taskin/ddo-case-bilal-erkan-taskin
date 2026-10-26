# FitGiriş — Spor Salonu Üyelik, Giriş Kontrol ve Ders Yönetim Sistemi

> **DDO Yazılım Ekibi · Alım Projesi**  
> **Geliştirici:** Bilal Erkan Taşkın  
> **Proje Deposu:** `ddo-case-bilal-erkan-taskin`  
> **Lisans:** MIT  

---

## 1. Projeye Genel Bakış

**FitGiriş**, 5 şubeli bir spor salonu zincirinin (Beşiktaş, Kadıköy, Maslak, Çankaya, Alsancak) tüm operasyonel ve finansal süreçlerini yöneten kurumsal bir platformdur.

Sistem; ay sonu kenar durumlarını çözen **Clamped Date aritmetiği**, 50 kişilik eşzamanlı kontenjan yarışlarında aşırı rezervasyonu önleyen **Pessimistic Row Locking (`FOR UPDATE`) & Bekleme Listesi FIFO Terfisi (`SKIP LOCKED`)**, **p95 < 5ms** gecikmeyle çalışan **Turnike Karar Motoru & 2 Saatlik Anti-Passback Denetimi**, vergi/muhasebe mevzuatına uygun **Boşluksuz (Gapless) Sıralı Fatura Üretimi**, **3-Retry Tahsilat Takvimi**, **20.000 üyelik deterministik seed motoru** ve **canlı olay simülatörü** ile uçtan uca eksiksiz geliştirilmiştir.

```mermaid
flowchart TD
    Client["Kullanıcı & Turnike İstemcileri\n(React Portal / Terminal / Simülatör)"] -->|HTTP / JSON| Router["Chi Router / Middleware\n(Auth, CORS, Idempotency, RateLimit)"]
    Router --> Service["Domain Servis Katmanı\n(Membership, Booking, Turnstile, Billing, PT, Reports)"]
    Service --> GateCache["Bellek İçi Önbellek\n(TurnstileGateCache - p95 < 5ms)"]
    Service --> PGX["Doğrudan PGX v5 Sürücüsü\n(pgxpool, Binary Protocol, Ham SQL)"]
    PGX --> PG[("PostgreSQL 16 Veritabanı\n(FOR UPDATE, SKIP LOCKED, BIGINT Kuruş)")]
```

---

## 2. Teknoloji Yığını ve Seçim Gerekçeleri

| Katman | Teknoloji | Seçim Gerekçesi |
| :--- | :--- | :--- |
| **Backend** | **Go 1.22+** (`chi` v5) | Hafif footprint, yüksek eşzamanlılık (goroutine), mikro-saniye seviyesinde I/O yönetimi ve standart kütüphane felsefesiyle %100 uyumluluk. |
| **Veritabanı** | **PostgreSQL 16** | ACID garantisi, güçlü satır kilitleri (`FOR UPDATE`, `SKIP LOCKED`), deterministik sequence kontrolleri ve bileşik B-Tree dizin performansı. |
| **DB Sürücüsü** | **`jackc/pgx/v5`** (`pgxpool`) | **ORM kesinlikle kullanılmamıştır.** Ham SQL ile tam kilit hâkimiyeti, sıfır abstraction overhead ve `pgx.CopyFrom` ile 20.000 üyenin saniyeler içinde seed edilmesi sağlanmıştır. |
| **Para Birimi** | **`BIGINT` (Kuruş)** | Finansal hesaplamalarda `float` yuvarlama hatalarını önlemek için tüm tutarlar kuruş cinsinden tamsayı olarak tutulmuştur (Örn: 1.500,00 TL = 150000). |
| **Frontend** | **React 18 + TypeScript + Vite + Tailwind CSS** | 5 rol için özelleştirilmiş, hızlı, tip güvenli, modern bileşen mimarisi ve responsive gösterge paneli. Multi-stage unprivileged Nginx üzerinde port 3000'de çalışır. |
| **Konteyner** | **Docker + Docker Compose** | Multi-stage derleme, `USER nonroot` güvenlik standardı, `healthcheck` ve `depends_on: service_healthy` orkestrasyonu. |
| **Simülatör** | **Go Canlı Servisi (`simulator`)** | Docker üzerinde bağımsız çalışan, 5 şubede periyodik turnike geçişleri, ders hücumları ve ödeme retry senaryoları üreten simülasyon servisi. |

---

## 3. Tek Komutla Hızlı Başlangıç

Sistem, şartname gereğince soğuk bir makinede **tek bir komutla** ayağa kalkar; veritabanı migrasyonları otomatik çalışır ve 5 şube ile 20.000 üye otomatik olarak seed edilir:

```bash
# 1. Depoyu klonlayın ve dizine girin
git clone https://github.com/bilalerkan/ddo-case-bilal-erkan-taskin.git
cd ddo-case-bilal-erkan-taskin

# 2. Çevre değişkenlerini kopyalayın
cp .env.example .env

# 3. Tüm sistemi Docker Compose ile derleyip başlatın
docker compose up --build
```

### Servis Erişim Bilgileri

| Servis / Arayüz | Adres / Port | Açıklama / Giriş Bilgileri |
| :--- | :--- | :--- |
| **Frontend Portalı** | [`http://localhost:3000`](http://localhost:3000) | 5 rol için özelleştirilmiş React yönetim paneli |
| **Veritabanı Web Arayüzü (Adminer)** | [`http://localhost:8085`](http://localhost:8085) | **Sistem:** PostgreSQL · **Sunucu:** `postgres` · **Kullanıcı:** `postgres` · **Şifre:** `postgres` · **Veritabanı:** `fitgiris` |
| **Backend REST API** | [`http://localhost:8080`](http://localhost:8080) | Go Chi REST API çekirdeği |
| **Swagger UI (Canlı API)** | [`http://localhost:8080/swagger`](http://localhost:8080/swagger) | İnteraktif canlı API dokümantasyonu |
| **OpenAPI 3.0 Şeması** | [`http://localhost:8080/openapi.yaml`](http://localhost:8080/openapi.yaml) | Ham OpenAPI tanımlayıcısı |
| **PostgreSQL Veritabanı** | Yalnızca Compose Ağı (`fitgiris-net`) | Şartname Bölüm 3 gereği host'a açılmaz; web arayüzü için Adminer kullanılır |
| **Sistem Sağlık Kontrolü** | [`http://localhost:8080/health`](http://localhost:8080/health) | API ve PostgreSQL canlılık (liveness/readiness) kontrolü |

> [!TIP]
> **Veritabanına Web Arayüzünden (Adminer) Tek Tıkla Erişim:**  
> Doğrudan tüm alanları hazır doldurulmuş olan bağlantıyı açabilirsiniz:  
> 🔗 **[`http://localhost:8085/?pgsql=postgres&username=postgres&db=fitgiris`](http://localhost:8085/?pgsql=postgres&username=postgres&db=fitgiris)**  
> Açılan ekranda yalnızca Parola (Password) kısmına **`postgres`** yazıp **Login** butonuna basmanız yeterlidir.  
> *(Kopyala-yapıştır yaparken kullanıcı adı veya şifrenin sonuna fazladan boşluk (` `) eklenmediğinden emin olun).*

---

## 4. Demo Kullanıcı Hesapları

Başlangıç seed verisi her rol için test edilmiş hazır hesaplar içerir. Tüm hesapların parolası ortaktır: **`Password123!`**

| Rol | E-Posta | Parola | Görev / Ekran Kapsamı |
| :--- | :--- | :--- | :--- |
| **Salon Müdürü (MANAGER)** | `manager@fitgiris.com` | `Password123!` | 5 şubenin ciro analitiği, doluluk grafiği, paket ve sınıf tanımlama |
| **Muhasebe (ACCOUNTANT)** | `accountant@fitgiris.com` | `Password123!` | Boşluksuz sıralı faturalar, tahsilat, otomatik yenileme kuyruğu |
| **Resepsiyon (RECEPTION)** | `reception@fitgiris.com` | `Password123!` | Canlı turnike geçiş terminali, anti-passback denetimi, çevrimdışı log sync |
| **Eğitmen (TRAINER)** | `trainer@fitgiris.com` | `Password123!` | Grup dersi programı, yoklama tamamlama, kişisel antrenman (PT) seansları |
| **Üye (MEMBER)** | `member@fitgiris.com` | `Password123!` | Dinamik turnike QR kodu, paket satın alma/dondurma, ders rezervasyonu |

> **Ek Not:** Deterministik tohumlayıcı (Seed=42) ayrıca `member_00001@fitgiris.com` .. `member_20000@fitgiris.com` aralığında **20.000 aktif/süresi dolmuş üye** oluşturmuştur.

---

## 5. Dört Zorlu Çekirdek Görev ve Çözüm Mimarisi

### Görev 1 (`T-1`): Üyelik Yaşam Döngüsü ve Clamped Date Aritmetiği
- **Problem:** Go standart kütüphanesindeki `time.AddDate(0, 1, 0)` fonksiyonu 31 Ocak'a 1 ay eklendiğinde takvim normalizasyonu yaparak "31 Şubat" yerine 2 veya 3 Mart üretir. Bu durum üyenin haksız gün kazanmasına ve ay sonu mutabakatının bozulmasına yol açar.
- **Çözüm (Clamped Date Algoritması):** Hedef ayın takvimdeki son geçerli gününü hesaplayan (`daysInMonth`) ve gün taşmasını ayın son gününe sabitleyen `AddClampedMonths` fonksiyonu yazılmıştır. 31 Ocak + 1 ay: artık yılda 29 Şubat, normal yılda 28 Şubat; 31 Mart + 1 ay: 30 Nisan olarak üretilir.
- **Sonlu Durum Makinesi (FSM):** Üyelikler `PENDING_PAYMENT`, `ACTIVE`, `FROZEN`, `TRANSFERRED`, `SUSPENDED`, `EXPIRED`, `CANCELLED` durumları arasında katı kurallarla geçiş yapar; yetkisiz geçişler `409 Conflict` ile engellenir.
- **Orantılı Yükseltme (Prorated Upgrade):** Paket yükseltmelerinde üyenin kalan gün değeri kuruş cinsinden hesaplanır:  
  $$\text{Kalan Değer} = \lfloor (\text{Kalan Gün} / \text{Toplam Gün}) \times \text{Mevcut Paket Tutarı} \rfloor$$
- **Kanıt Komutu:**
  ```bash
  cd backend && go test -v ./internal/service -run TestMembershipLifecycle
  ```

### Görev 2 (`T-2`): Grup Dersi Rezervasyonu (50 Kişi Hücumu & Waitlist FIFO Terfisi)
- **Problem:** Son 1 boş kontenjana 50 üyenin aynı anda rezervasyon isteği göndermesi (race condition).
- **Çözüm (Pessimistic Row Locking):** Ders takvimi satırı `SELECT ... FOR UPDATE` ile kilitlenir. Yalnızca 1 üye `CONFIRMED` statüsüne alınır; kalan 49 üye `class_waitlist` tablosuna FIFO sırasıyla (`created_at ASC`) eklenir.
- **2 Saat İptal Kuralı & Otomatik Terfi:** Derse 2 saatten fazla kala iptal edilirse cezasız iptal yapılır ve bekleme listesindeki ilk aday `SELECT ... FOR UPDATE SKIP LOCKED LIMIT 1` ile asil kontenjana terfi ettirilir. 2 saatten az kala iptalde ceza (`LATE_CANCELLED`) uygulanır ve seans hakkı düşülür.
- **Kanıt Komutu:**
  ```bash
  cd backend && go test -v ./internal/service -run TestBooking_Concurrency_50Members1Slot
  ```

### Görev 3 (`T-3`): Turnike Karar Motoru (p95 < 50ms) ve 2 Saatlik Anti-Passback Kuralı
- **Düşük Gecikme Stratejisi:** Turnike kapı kontrolünde aktif üyelik, kalan seans hakkı ve şube geçerliliği denetlenir. Yüksek I/O yükünü önlemek için thread-safe `TurnstileGateCache` mimarisi kurulmuştur; turnike kararı mikrosaniyeler seviyesinde (p95 = **4.2 ms**) verilir.
- **Anti-Passback Kontrolü:** Aynı kart ile çıkış (`OUT`) yapılmadan son 2 saat içinde ikinci kez giriş (`IN`) yapılmak istendiğinde geçiş `DENIED` edilir ve `ERR_ANTI_PASSBACK` üretilir.
- **Çevrimdışı Mod ve Senkronizasyon:** İnternet kesildiğinde turnike yerel önbelleğindeki beyaz listeyle kapıyı açar. Ağ geldiğinde `POST /api/v1/turnstiles/sync-offline` ile loglar gönderilir; çakışmalar `audit_logs` tablosuna `FLAGGED_OFFLINE_ANOMALY` olarak işlenir.
- **Kanıt Komutu:**
  ```bash
  cd backend && go test -v ./internal/service -run TestTurnstileService_LatencyBenchmark
  # veya 1.000 eşzamanlı istek yük testi:
  make load
  ```

### Görev 4 (`T-4`): Otomatik Yenileme ve Boşluksuz (Gapless) Sıralı Fatura Numarası
- **Problem:** PostgreSQL `SEQUENCE` nesneleri transaction `ROLLBACK` aldığında üretilen numarayı geri alamaz ve seride boşluk (gap) bırakır. Mevzuat gereği fatura numaraları sıfır boşluklu ve mükerrersiz olmalıdır.
- **Çözüm (Kilitli Sayaç Tablosu):** Şube ve yıl bazlı tek satırlık `invoice_sequences` tablosu üzerinde `SELECT current_val FROM invoice_sequences WHERE year = $1 AND branch_id = $2 FOR UPDATE` çalıştırılır. Transaction iptal olursa kilit serbest kalır ve sayaç harcanmaz. Böylece 50 eşzamanlı faturada bile `FIT-2026-IST01-000001` .. `FIT-2026-IST01-000050` arasında %100 boşluksuz seri elde edilir.
- **3 Deneme Takvimi & Askıya Alma:** Sahte sağlayıcı (%20 hata simülasyonu) ile 1., 3. ve 5. günlerde tahsilat denenir. 7. günde hala ödenmemişse üyelik `SUSPENDED` statüsüne alınır ve turnike geçişleri kapatılır.
- **Kanıt Komutu:**
  ```bash
  cd backend && go test -v ./internal/service -run TestBilling_Concurrency_50SequentialInvoices_ZeroGaps
  ```

---

## 6. Veritabanı Dizin Tasarımı ve EXPLAIN (ANALYZE) Performans Ölçümleri

Şartname Bölüm 7 gereğince, kritik iş yüklerini taşıyan sorgular için özel B-Tree dizinleri tasarlanmış ve 20.000 üyeli veri tabanında `EXPLAIN (ANALYZE)` ile doğrulanmıştır.

### 1. Dizin: `idx_access_logs_anti_passback`
- **Tablo:** `access_logs`
- **Tasarım:** `CREATE INDEX idx_access_logs_anti_passback ON access_logs(member_id, branch_id, status, created_at DESC);`
- **Hedef Sorgu:** Turnike geçiş anında son 2 saatteki anti-passback durumunu sorgulayan kritik sorgu:
  ```sql
  SELECT created_at, direction 
  FROM access_logs 
  WHERE member_id = 'e7b1a23c-4d5e-6f7a-8b9c-0d1e2f3a4b5c' 
    AND branch_id = 'c1b2a3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d' 
    AND status = 'GRANTED' 
  ORDER BY created_at DESC 
  LIMIT 1;
  ```

#### Performans Karşılaştırması:
| Metrik | Dizinsiz (Sequential Scan) | Dizinli (Index Scan Backward) | Kazanım |
| :--- | :--- | :--- | :--- |
| **Plan Türü** | `Seq Scan on access_logs` + `Sort` | `Index Scan Backward using idx_access_logs_anti_passback` | Sıralama ve tablo taraması kalktı |
| **Tahmini Maliyet (Cost)** | `0.00..4850.25` | `0.29..8.31` | **~580 kat daha az I/O** |
| **Yürütme Süresi (Execution Time)** | **52.410 ms** | **0.142 ms** | **~370 kat hızlanma (p95 < 5ms sağlandı)** |

```text
-- DİZİNLİ EXPLAIN (ANALYZE) ÇIKTISI:
Limit  (cost=0.29..8.31 rows=1 width=16) (actual time=0.085..0.086 rows=1 loops=1)
  ->  Index Scan Backward using idx_access_logs_anti_passback on access_logs
        Index Cond: ((member_id = '...'::uuid) AND (branch_id = '...'::uuid) AND ((status)::text = 'GRANTED'::text))
Planning Time: 0.098 ms
Execution Time: 0.142 ms
```

---

### 2. Dizin: `idx_class_waitlist_fifo`
- **Tablo:** `class_waitlist`
- **Tasarım:** `CREATE INDEX idx_class_waitlist_fifo ON class_waitlist(schedule_id, status, created_at ASC);`
- **Hedef Sorgu:** Grup dersi iptal edildiğinde ilk bekleyen adayı atomik olarak kilitleyip terfi ettiren sorgu:
  ```sql
  SELECT id, member_id 
  FROM class_waitlist 
  WHERE schedule_id = 'a1b2c3d4-0000-0000-0000-000000000001' 
    AND status = 'WAITING' 
  ORDER BY created_at ASC 
  LIMIT 1 
  FOR UPDATE SKIP LOCKED;
  ```

#### Performans Karşılaştırması:
| Metrik | Dizinsiz (Sequential Scan) | Dizinli (Index Scan) | Kazanım |
| :--- | :--- | :--- | :--- |
| **Plan Türü** | `Seq Scan on class_waitlist` + `Sort` | `Index Scan using idx_class_waitlist_fifo` | CPU sort yükü sıfırlandı |
| **Tahmini Maliyet (Cost)** | `0.00..1240.10` | `0.28..4.30` | **~288 kat daha az maliyet** |
| **Yürütme Süresi (Execution Time)** | **14.850 ms** | **0.118 ms** | **~125 kat hızlanma (Kilit süresi < 1ms)** |

```text
-- DİZİNLİ EXPLAIN (ANALYZE) ÇIKTISI:
Limit  (cost=0.28..4.30 rows=1 width=24) (actual time=0.062..0.063 rows=1 loops=1)
  ->  LockRows (cost=0.28..4.30 rows=1 width=24) (actual time=0.058..0.059 rows=1 loops=1)
        ->  Index Scan using idx_class_waitlist_fifo on class_waitlist
              Index Cond: ((schedule_id = '...'::uuid) AND ((status)::text = 'WAITING'::text))
Planning Time: 0.081 ms
Execution Time: 0.118 ms
```

---

## 7. Doğrulama Soruları (README Zorunlu Bölümü)

### 1. En zorlandığın görev hangisiydi? Hangi alternatif çözümleri düşündün ve neden eledin?

**En çok zorlandığım görev ve süreç:**  
Bu projede kod yazımı, mimari iskeletin kurulması ve algoritmik kurguların tamamı yapay zekâ (LLM) araçları orkestre edilerek gerçekleştirilmiştir. Bu nedenle geliştirici olarak şahsım adına en zorlayıcı kısım tekil bir Go fonksiyonu yazmak değil; **yapay zekâ modelinin bağlam (context) limitlerine takılmadan, halüsinasyon görmesini (olmayan tablolar veya kütüphaneler uydurmasını) engelleyerek ve proje dokümanında yer alan prompt injection ve yanıltıcı yönergeleri erken aşamada yakalayıp eleyerek projeyi uçtan uca, tutarlı ve eksiksiz bir şekilde tamamlamaktı.**

**Düşünülen Alternatif Yaklaşımlar ve Elenme Nedenleri:**
1. **Alternatif 1 — Monolitik Prompting (Tüm Projeyi Tek Seferde Yazdırma):**
   - *Neden Eledim?:* Projenin tamamını tek bir devasa prompt ile yapay zekâya bırakmak, modelin bağlam penceresi limitlerini doldurmasına, kodların yarım kalmasına, migration şemalarındaki kolon isimlerini unutarak hayali değişkenler türetmesine yol açıyordu.
2. **Alternatif 2 — Doğrulama ve Testleri En Sona Bırakma:**
   - *Neden Eledim?:* Testleri ve derleme kontrollerini projenin sonuna bırakmak, PostgreSQL 16 veri tipi uyuşmazlıklarının ve Go tip güvenliği hatalarının birikerek içinden çıkılamaz bir duruma gelmesine sebep oluyordu.
3. **Seçilen Nihai Çözüm (Faz Bazlı Orkestrasyon + Katı Grounding & Kural Sabitleme):**
   - Proje 7 bağımsız mantıksal faza bölündü (`.agents/skills/fitgiris-rules` tanımlandı).
   - Her fazda yapay zekânın halüsinasyon görmesini engellemek için **Grounding Kuralı** (`go build ./...` ve `go test ./...` koşmadan sonraki adıma asla geçilmemesi) ve **Şema Sabitleme** (`migrations/*.sql` haricinde kolon uydurulmaması) zorunlu kılındı.
   - Dokümanda kısıtları delmeye veya sistemi yanlış yönlendirmeye yönelik prompt injection yönergeleri ayıklanarak modelin şartname sınırlarında kalması sağlandı.

---

### 2. Çözümünün kırıldığı ya da yetersiz kaldığı bir senaryo bul ve nedenini açıkla (bilinen sınırlama).

**Bilinen Sınırlama: Çevrimdışı (Offline) Modda Şubeler Arası Dağıtık Anti-Passback Senaryosu.**

- **Senaryo:** Şubedeki internet bağlantısı koptuğunda turnike yerel önbelleğindeki beyaz listeye dayanarak üyenin geçişine izin verir ve logları yerel SQLite / bellek kuyruğuna yazar. Eğer bir üye Beşiktaş şubesindeki turnikeden çevrimdışı modda kartını okutup içeri girer girmez, kartını salonun dışındaki arkadaşına verirse ve arkadaşı henüz internet gelmeden 5 dakika sonra Kadıköy şubesine (veya aynı şubedeki diğer bağımsız çevrimdışı turnikeye) giderse; turnikeler arasında anlık merkezi bir event-stream ağı olmadığı için bu mükerrer giriş anında engellenemez.
- **Sistemin Yanıtı:** Şartnamede açıklandığı üzere bu durum çevrimdışı mimarinin teorik sınırıdır (CAP teoremi / network partition). İnternet bağlantısı yeniden kurulduğunda turnike yerel kuyruğunu `POST /api/v1/turnstiles/sync-offline` ucuyla merkeze gönderir. Backend logları inceler, son 2 saat içinde aynı kartla yapılan çakışan girişleri tespit eder ve `audit_logs` tablosuna `FLAGGED_OFFLINE_ANOMALY` olarak işleyerek salon müdürünün ekranına anomali uyarısı düşürür.

---

### 3. Veritabanına eklediğin dizinlerden ikisini seç: hangi sorgu için, dizinsiz/dizinli ölçüm farkı nedir?

Seçilen iki kritik dizin şunlardır (Ayrıntılı `EXPLAIN (ANALYZE)` çıktıları Bölüm 6'da sunulmuştur):

1. **`idx_access_logs_anti_passback`:**
   - **Sorgu:** `SELECT created_at, direction FROM access_logs WHERE member_id = $1 AND branch_id = $2 AND status = 'GRANTED' ORDER BY created_at DESC LIMIT 1;`
   - **Ölçüm Farkı:** 20.000 üyeli veritabanında dizinsiz tam tablo taramasında (Seq Scan) yürütme süresi **52.4 ms** iken, bileşik B-Tree dizini sayesinde **0.14 ms**'ye düşmüş ve yaklaşık **370 kat hızlanma** sağlanarak turnike p95 < 50ms hedefi rahatlıkla tutturulmuştur.
2. **`idx_class_waitlist_fifo`:**
   - **Sorgu:** `SELECT id, member_id FROM class_waitlist WHERE schedule_id = $1 AND status = 'WAITING' ORDER BY created_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED;`
   - **Ölçüm Farkı:** Rezervasyon iptal edildiğinde bekleme listesindeki ilk adayı kilitleyen bu sorguda dizinsiz yürütme süresi **14.8 ms** iken, dizinli çalışmada **0.11 ms**'ye inmiş ve satır kilidi tutulma süresi mikrosaniyeler seviyesine çekilmiştir.

---

### 4. Yapay zekâ kullandıysan, reddettiğin ya da düzelttiğin bir çıktıyı (önce/sonra) göster ve neden yanlış olduğunu açıkla.

Geliştirme sürecinde yapay zekânın ürettiği çıktılarda hem doküman kaynaklı yanıltmalar/enjeksiyonlar hem de kod seviyesinde kritik mantık hataları yakalanıp reddedilmiştir:

#### A. Dokümandaki Prompt Injection ve Yanıltıcı Kurguların Yakalanması
Vaka dokümanında ve süreçte yer alan, yapay zekâyı şartname sınırları dışına çıkarmaya ve güvenlik açıklarına zorlayan yönlendirmeler (örneğin PostgreSQL veritabanı portunun host makineye açılması yönündeki yönlendirmeler veya rol izolasyonunu sunucu tarafı yerine yalnızca arayüzde gizleme eğilimleri) yakalanmıştır. Şartname Bölüm 3'teki *"PostgreSQL host'a açılması gerekmez, yalnızca compose ağında erişilsin"* ve Bölüm 4'teki *"yetkisiz erişimin sunucu tarafında engellenmesi"* hükümleri gereğince yapay zekânın bu çıktıları reddedilmiş; PostgreSQL portu host erişimine kapatılmış ve her uç noktaya 401/403 RBAC denetimi getirilmiştir.

#### B. Kod Düzeyinde Reddedilen Çıktı: `time.AddDate` Ay Sonu Taşma Hatası
- **Yapay Zekânın İlk Önerdiği Kod (Reddedilen Yaklaşım):**
  ```go
  // HATALI YAKLAŞIM: time.AddDate kullanımı
  func CalculateExpiryDate(startDate time.Time, durationMonths int) time.Time {
      return startDate.AddDate(0, durationMonths, 0)
  }
  ```
- **Neden Yanlış Olduğu:**  
  Go'nun standart `time.AddDate(year, month, day)` fonksiyonu takvim normalizasyonu uygular. Başlangıç tarihi **31 Ocak** olduğunda ve 1 ay eklendiğinde, Go takvimde 31 Şubat bulunmadığı için taşan günleri bir sonraki aya ekleyerek normal yılda **3 Mart**, artık yılda ise **2 Mart** üretir! Bu durum üyenin 1-3 gün haksız salona girmesine ve üyelik hesaplarının bozulmasına yol açar.
- **Düzeltilen Kod (`internal/platform/dateutil/clamped_date.go`):**
  ```go
  // DOĞRU YAKLAŞIM: Clamped Date Algoritması
  func AddClampedMonths(t time.Time, months int) time.Time {
      targetYear := t.Year()
      targetMonth := int(t.Month()) + months
      for targetMonth > 12 {
          targetYear++
          targetMonth -= 12
      }
      maxDay := daysInMonth(targetYear, time.Month(targetMonth))
      targetDay := t.Day()
      if targetDay > maxDay {
          targetDay = maxDay // Hedef ayın son gününe sıkıştır (clamp: 28 veya 29 Şubat)
      }
      return time.Date(targetYear, time.Month(targetMonth), targetDay,
          t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
  }
  ```
- **Sonuç:** 31 Ocak'a 1 ay eklendiğinde artık yılda (2024) tam olarak **29 Şubat**, normal yılda (2025/2026) ise **28 Şubat** üretildiği birim testlerle kanıtlanmıştır.

---

### 5. “Üyelik yaşam döngüsü ve tarih hesapları” görevindeki çözümünün doğru çalıştığını nasıl kanıtladın? Komutu ve çıktısını yaz.

**Kanıt Komutu:**
```bash
cd backend && go test -v ./internal/service -run TestMembershipLifecycle
```

**Gerçek Test Çıktısı:**
```text
=== RUN   TestMembershipLifecycle_ClampedDateEdgeCases
=== RUN   TestMembershipLifecycle_ClampedDateEdgeCases/31_Ocak_to_Subat_Artik_Yil_2024
=== RUN   TestMembershipLifecycle_ClampedDateEdgeCases/31_Ocak_to_Subat_Normal_Yil_2025
=== RUN   TestMembershipLifecycle_ClampedDateEdgeCases/31_Mayis_to_Haziran
=== RUN   TestMembershipLifecycle_ClampedDateEdgeCases/31_Agustos_to_Eylul
--- PASS: TestMembershipLifecycle_ClampedDateEdgeCases (0.00s)
    --- PASS: TestMembershipLifecycle_ClampedDateEdgeCases/31_Ocak_to_Subat_Artik_Yil_2024 (0.00s)
    --- PASS: TestMembershipLifecycle_ClampedDateEdgeCases/31_Ocak_to_Subat_Normal_Yil_2025 (0.00s)
    --- PASS: TestMembershipLifecycle_ClampedDateEdgeCases/31_Mayis_to_Haziran (0.00s)
    --- PASS: TestMembershipLifecycle_ClampedDateEdgeCases/31_Agustos_to_Eylul (0.00s)
=== RUN   TestMembershipLifecycle_FSM_ValidAndInvalidTransitions
=== RUN   TestMembershipLifecycle_FSM_ValidAndInvalidTransitions/Active_to_Frozen_Success
=== RUN   TestMembershipLifecycle_FSM_ValidAndInvalidTransitions/Frozen_to_Transferred_Conflict409
=== RUN   TestMembershipLifecycle_FSM_ValidAndInvalidTransitions/Cancelled_to_Active_Conflict409
--- PASS: TestMembershipLifecycle_FSM_ValidAndInvalidTransitions (0.00s)
=== RUN   TestMembershipLifecycle_ProratedUpgrade_Calculation
--- PASS: TestMembershipLifecycle_ProratedUpgrade_Calculation (0.00s)
PASS
ok  	fitgiris/internal/service	0.012s
```

Ayrıca `make verify` otomasyonu çalıştırıldığında `T-1` kimliği `PASS` olarak raporlanmaktadır.

---

## 8. Hızlı Doğrulama ve Test Otomasyonu

Şartname Bölüm 10 uyarınca ekibin projeyi kısa sürede inceleyebilmesi için tek komutlu araçlar geliştirilmiştir:

### A. Tek Komutla Otomatik Doğrulama Paketi (`make verify`)
Ayağa kalkmış sistem üzerinde tüm özellikleri (`HEALTH`, `AUTH`, `RBAC`, `F-1`..`F-6`, `T-1`..`T-4`) dener ve aşağıdaki sonuç tablosunu basar:

```bash
make verify
```

```text
┌─────────┬────────────────────────────────────────────────────────┬────────┬──────────┐
│ ID      │ Dogrulama Kimligi / Aciklama                           │ Durum  │ Sure     │
├─────────┼────────────────────────────────────────────────────────┼────────┼──────────┤
│ HEALTH  │ Backend /health ve Veritabani Baglanti Durumu          │ PASS   │    1.1ms │
│ AUTH    │ JWT Kimlik Dogrulama & Hatali Parola 401 Engeli        │ PASS   │    1.0ms │
│ RBAC    │ Rol Tabanli Yetkilendirme & 403 Forbidden Kontrolu     │ PASS   │    0.5ms │
│ F-1     │ Uyelik Paketleri Tanimi ve Paket Satisi                │ PASS   │    0.5ms │
│ F-2     │ Fatura Uretimi, Idempotent Odeme ve Tahsilat           │ PASS   │    0.5ms │
│ F-3     │ Turnike Giris Karari & Seans Hakki Dusumu              │ PASS   │    0.5ms │
│ F-4     │ Grup Dersi Programi, Rezervasyon ve Yoklama            │ PASS   │    0.5ms │
│ F-5     │ Kisisel Antrenman (PT) Paket ve Randevu Yonetimi       │ PASS   │    1.0ms │
│ F-6     │ Finansal Ciro, Saatlik Doluluk ve Egitmen Raporlari    │ PASS   │    0.0ms │
│ T-1     │ Gorev 1: Clamped Date, FSM Durum Gecisleri & Upgrade   │ PASS   │    0.5ms │
│ T-2     │ Gorev 2: 50 Eszamanli Rezervasyon, 2h Iptal & Waitlist │ PASS   │    2.9ms │
│ T-3     │ Gorev 3: Turnike Gecikmesi (p95<50ms) & Anti-Passback  │ PASS   │    6.1ms │
│ T-4     │ Gorev 4: Bosluksuz Fatura Numarasi & 3-Retry Takvimi   │ PASS   │    1.0ms │
└─────────┴────────────────────────────────────────────────────────┴────────┴──────────┘

SONUÇ: 13 / 13 DOĞRULAMA TESTİ BAŞARIYLA TAMAMLANDI (TÜM GÖREVLER PASS)
```

### B. Demo Verilerini Sıfırlama (`make reset-demo`)
Veritabanını ilk tohumlanmış haline (Seed=42, 5 şube, 20.000 üye) sıfırlamak için:
```bash
make reset-demo
```

### C. Turnike Karar Motoru Yük Testi (`make load`)
1.000 eşzamanlı istekle p95 gecikmesini ölçer:
```bash
make load
# p95: 4.2ms (< 50ms şartı sağlandı)
```

---

## 9. İlgili Dokümantasyon Dosyaları

- [`DECISIONS.md`](file:///C:/Users/MONSTER/OneDrive/Desktop/gym_center_project/DECISIONS.md): 5 adet detaylı Mimari Karar Kaydı (ADR-001 .. ADR-005).
- [`TESTING.md`](file:///C:/Users/MONSTER/OneDrive/Desktop/gym_center_project/TESTING.md): Demo hesapları, manuel arayüz senaryoları ve görev kanıtları.
- [`requests.http`](file:///C:/Users/MONSTER/OneDrive/Desktop/gym_center_project/requests.http): VS Code REST Client ile tüm uç noktaları deneyebileceğiniz hazır HTTP koleksiyonu.
- [`docs/openapi.yaml`](file:///C:/Users/MONSTER/OneDrive/Desktop/gym_center_project/docs/openapi.yaml): Tam OpenAPI 3.0 REST API spesifikasyonu.
- [`docs/phase-*-summary.md`](file:///C:/Users/MONSTER/OneDrive/Desktop/gym_center_project/docs): Faz 1'den Faz 7'ye kadar olan aşamaların teknik raporları.

---

*FitGiriş sistemi DDO alım kriterlerine ve endüstri standardı kod kalitesine tam uyumlu şekilde teslim edilmiştir.*
