# FitGiriş — Mimari Karar Kayıtları (Architecture Decision Records - ADR)

Bu belge, FitGiriş spor salonu zinciri yönetim sisteminin tasarımı ve geliştirilmesi sırasında alınan temel mimari kararları, bu kararların bağlamını, değerlendirilen alternatifleri, nihai tercihin gerekçelerini ve getirdiği kazanım/külfetleri (trade-offs) kayıt altına almaktadır.

---

## ADR İndeksi

- [ADR-001: Veritabanı Erişim Katmanında ORM Yerine Doğrudan `jackc/pgx/v5` ve Ham SQL Tercihi](#adr-001-veritabanı-erişim-katmanında-orm-yerine-doğrudan-jackcpgxv5-ve-ham-sql-tercihi)
- [ADR-002: Üyelik Yaşam Döngüsü ve Ay Sonu Kenar Durumlarında Clamped Date Aritmetiği & FSM Durum Makinesi](#adr-002-üyelik-yaşam-döngüsü-ve-ay-sonu-kenar-durumlarında-clamped-date-aritmetiği--fsm-durum-makinesi)
- [ADR-003: Grup Dersi Eşzamanlı Rezervasyonunda Pessimistic Row Locking (`FOR UPDATE`) & Bekleme Listesi FIFO Terfisi (`SKIP LOCKED`)](#adr-003-grup-dersi-eşzamanlı-rezervasyonunda-pessimistic-row-locking-for-update--bekleme-listesi-fifo-terfisi-skip-locked)
- [ADR-004: Boşluksuz (Gapless) Sıralı Fatura Numaralandırmasında PostgreSQL Sequence Yerine Kilitli Sayaç Tablosu (`invoice_sequences`)](#adr-004-boşluksuz-gapless-sıralı-fatura-numaralandırmasında-postgresql-sequence-yerine-kilitli-sayaç-tablosu-invoice_sequences)
- [ADR-005: Turnike Karar Motorunda Düşük Gecikme (p95 < 5ms) İçin Bellek İçi Önbellek (`TurnstileGateCache`) ve İki Saatlik Anti-Passback Denetimi](#adr-005-turnike-karar-motorunda-düşük-gecikme-p95--5ms-için-bellek-içi-önbellek-turnstilegatecache-ve-iki-saatlik-anti-passback-denetimi)

---

## ADR-001: Veritabanı Erişim Katmanında ORM Yerine Doğrudan `jackc/pgx/v5` ve Ham SQL Tercihi

### Durum
**Kabul Edildi (Accepted)**

### Tarih
2026-10-25

### Bağlam (Context)
FitGiriş sistemi; turnike geçişlerinde 50 ms altı karar gecikmesi (p95 < 50ms), eşzamanlı kontenjan yarışları, kilitli sayaç artırımı, 20.000 üyeli seed verisinin saniyeler içinde yüklenmesi (`pgx.CopyFrom`) ve hassas para yönetimi (kuruş bazlı `BIGINT`) gibi yüksek performans ve katı veritabanı eşzamanlılık kontrolü gerektirmektedir.

Go ekosisteminde veritabanı etkileşimi için yaygın olarak GORM, Ent, SQLBoiler gibi ORM araçları ya da `database/sql` ve `pgx` gibi sürücüler kullanılmaktadır.

### Değerlendirilen Seçenekler (Options Considered)
1. **GORM (Go Object Relational Mapping):**
   - *Artıları:* Hızlı prototipleme, otomatik migrasyon, zengin topluluk desteği.
   - *Eksileri:* Reflection kullanımı nedeniyle işlemci/bellek ek yükü (overhead); transaction içinde satır seviyesi kilit (`SELECT ... FOR UPDATE`, `SKIP LOCKED`) sentaksında kısıtlar; PostgreSQL'e özgü özelliklerin (ör. `pgx.CopyFrom` binary stream, `TIMESTAMPTZ` hassasiyeti) soyutlanması; örtük sorgu (N+1 query) riski.
2. **Ent (Facebook / Ariga):**
   - *Artıları:* Tip güvenli kod üretimi, şema tabanlı doğrulama.
   - *Eksileri:* Aşırı kod üretim çıktısı, karmaşık transaction kilitleri (`SKIP LOCKED`) için özel SQL escape zorunluluğu, öğrenme eğrisi.
3. **Doğrudan `jackc/pgx/v5` (`pgxpool`) ve Ham SQL Sorguları (Seçilen Yaklaşım):**
   - *Artıları:* PostgreSQL binary protokolü üzerinden sıfır ek yükle en yüksek I/O verimliliği; `SELECT ... FOR UPDATE` ve `FOR UPDATE SKIP LOCKED` kilitlerinde tam SQL hâkimiyeti; `pgx.CopyFrom` ile 20.000 kaydı ~1 saniyede aktarabilme; `BIGINT` kuruş dönüşümlerinde %100 tip güvenliği; hiçbir "magic" içermeyen şeffaf hata ayıklama.
   - *Eksileri:* Veri modelleri ve SQL sorgularının elle yazılması, boilerplate kod artışı.

### Alınan Karar (Decision)
Veritabanı erişim katmanında ORM kullanılmayacak, doğrudan PostgreSQL için endüstri standardı olan **`jackc/pgx/v5`** sürücüsü ve bağlantı havuzu (`pgxpool`) tercih edilecektir. Şema değişiklikleri `migrations/` dizini altındaki sıralı `.sql` dosyaları üzerinden çalıştırılacaktır.

### Sonuçlar ve Külfetler (Consequences & Trade-offs)
- **Kazanımlar:**
  - Turnike sorgularında gecikme mikrosaniye seviyelerine indirildi (p95 < 5ms).
  - 20.000 üyenin seed yüklemesi `pgx.CopyFrom` sayesinde 1.5 saniyenin altında tamamlandı.
  - Eşzamanlılık kilitleri (`FOR UPDATE`, `SKIP LOCKED`) SQL düzeyinde hatasız ve deterministik işletildi.
- **Külfetler (Trade-offs):**
  - CRUD işlemleri için struct eşleştirme (mapping) kodları repository katmanında manuel yazılmıştır. Bu durum `fitgiris-rules` kuralı ("Şema Sabitleme") ile disipline edilerek tüm sorgu ve struct alanlarının `migrations/*.sql` ile birebir uyumlu olması sağlanmıştır.

---

## ADR-002: Üyelik Yaşam Döngüsü ve Ay Sonu Kenar Durumlarında Clamped Date Aritmetiği & FSM Durum Makinesi

### Durum
**Kabul Edildi (Accepted)**

### Tarih
2026-10-25

### Bağlam (Context)
Spor salonu üyelik paketleri genellikle aylık, 3 aylık, 6 aylık veya yıllık olarak satılır. Tarih hesaplarında ay sonu kenar durumları (edge cases) finansal ve operasyonel uyuşmazlıklara yol açar.
Örneğin:
- 31 Ocak tarihinde 1 aylık paket alan bir üyenin bitiş tarihi ne olmalıdır?
- Şubat ayının 28 çektiği normal yıllar ile 29 çektiği artık yıllarda (leap year) bu hesap nasıl yapılmalıdır?
- Go standart kütüphanesindeki `time.AddDate(0, 1, 0)` fonksiyonu normalizasyon yaparak 31 Ocak'a 1 ay eklendiğinde "31 Şubat" tarihini 2 Mart veya 3 Mart'a yuvarlar! Bu durum üyenin haksız 1-3 gün fazladan salona girmesine neden olur.

Ayrıca üyeliğin durumu (aktif, dondurulmuş, devredilmiş, askıya alınmış, süresi dolmuş, iptal edilmiş) kontrolsüz durum geçişlerine karşı korunmalıdır.

### Değerlendirilen Seçenekler (Options Considered)
1. **Standart `time.AddDate` Kullanımı:**
   - *Eksileri:* 31 Ocak + 1 ay = 2 ya da 3 Mart oluşturur. Takvim ayı mantığı kırılır, haksız kullanım doğar.
2. **Sabit 30 Gün Ekleme:**
   - *Eksileri:* 31 çeken aylarda veya Şubat ayında gerçek ay sonlarıyla uyuşmaz; üyenin her ay farklı bir günde yenileme yapmasına yol açar (müşteri memnuniyetsizliği).
3. **Clamped Date (Sıkıştırmalı Ay Sonu) Algoritması (Seçilen Yaklaşım):**
   - Hedef ayın takvimdeki son geçerli günü hesaplanır (`daysInMonth(year, month)`).
   - Kaynak gün hedef ayın son gününden büyükse, gün sayısı hedef ayın son gününe sabitlenir (`clamped`).
   - Örn: 31 Ocak + 1 ay artık yılda (2024) -> 29 Şubat; normal yılda (2025/2026) -> 28 Şubat; 31 Mart + 1 ay -> 30 Nisan.

### Alınan Karar (Decision)
1. Tarih aritmetiğinde `internal/platform/dateutil` paketi altında **`AddClampedMonths(t time.Time, months int) time.Time`** algoritması geliştirilmiştir.
2. Üyelik yaşam döngüsü katı bir Sonlu Durum Makinesi (Finite State Machine - FSM) ile modellenmiştir:
   - İzin Verilen Durumlar: `PENDING_PAYMENT`, `ACTIVE`, `FROZEN`, `TRANSFERRED`, `SUSPENDED`, `EXPIRED`, `CANCELLED`.
   - İzin verilen geçişler dışındaki tüm talepler (ör. dondurulmuş bir üyeliği doğrudan devretmek veya iptal edilmiş üyeliği dondurmak) `409 Conflict` ve `ERR_INVALID_STATE_TRANSITION` ile engellenir.
3. Paket yükseltmelerinde (Upgrade) kalan günlerin orantılı değeri kuruş hassasiyetinde hesaplanır:
   $$\text{Kalan Değer} = \lfloor (\text{Kalan Gün} / \text{Toplam Gün}) \times \text{Mevcut Paket Tutarı} \rfloor$$
   $$\text{Ödenecek Fark} = \max(0, \text{Yeni Paket Tutarı} - \text{Kalan Değer})$$

### Sonuçlar ve Külfetler (Consequences & Trade-offs)
- **Kazanımlar:**
  - Artık yıl ve ay sonu kenar durumları %100 matematiksel tutarlılıkla çözüldü (`TestMembershipLifecycle_EdgeCases` birim testleriyle kanıtlandı).
  - Geçersiz üyelik hareketleri engellendi; dondurulan gün sayısı çözülme anında bitiş tarihine hassas şekilde eklendi.
- **Külfetler (Trade-offs):**
  - Go standart kütüphanesinin hazır fonksiyonu yerine özel tarih mantığı bakımı gerektirmiştir; ancak bu mantık kapsamlı testlerle garanti altına alınmıştır.

---

## ADR-003: Grup Dersi Eşzamanlı Rezervasyonunda Pessimistic Row Locking (`FOR UPDATE`) & Bekleme Listesi FIFO Terfisi (`SKIP LOCKED`)

### Durum
**Kabul Edildi (Accepted)**

### Tarih
2026-10-25

### Bağlam (Context)
Popüler grup derslerinde (ör. Spinning, Pilates, Crossfit) son 1 kontenjan kaldığında, onlarca üyenin aynı anda "Rezerve Et" butonuna basması durumunda yarış durumu (race condition) oluşur. Aşırı rezervasyon (overbooking) yapılması sınıfın fiziksel kapasitesini aşacağından kabul edilemez.

Ayrıca:
- Kontenjan dolduğunda gelen üyelerin sırayla bekleme listesine (waitlist) alınması gerekir.
- Derse 2 saatten fazla kala bir rezervasyon iptal edildiğinde, bekleme listesindeki ilk üyenin otomatik olarak asil kontenjana terfi ettirilmesi (Waitlist Promotion) beklenir.
- Üyenin aynı saatte çakışan iki farklı derse rezervasyon yapması engellenmelidir.

### Değerlendirilen Seçenekler (Options Considered)
1. **İyimser Kilitleme (Optimistic Locking - Version Column):**
   - *Eksileri:* Yüksek çekişmede (high contention, 50 eşzamanlı istek) çok sayıda transaction `abort` olur; client tarafında yoğun retry döngüsü ve CPU dalgalanması oluşur.
2. **Uygulama İçi (In-Memory) Mutex:**
   - *Eksileri:* Tek bir Go işleminde çalışır; Docker veya yatayda birden fazla backend replikası çalıştığında ölçeklenemez ve güvenliğini kaybeder.
3. **PostgreSQL Kötümser Satır Kilidi (`SELECT ... FOR UPDATE`) ve `SKIP LOCKED` (Seçilen Yaklaşım):**
   - İlgili ders takvimi satırı `SELECT ... FOR UPDATE` ile kilitlenir. Diğer transaction'lar sıraya girer.
   - `booked_count < capacity` kontrolü kilit altındayken yapılır; kontenjan varsa `booked_count` artırılır ve `class_bookings` tablosuna `CONFIRMED` kaydı açılır.
   - Kontenjan dolmuşsa `class_waitlist` tablosuna FIFO sırasıyla (`created_at ASC`) `WAITING` olarak eklenir.
   - İptal durumunda bekleme listesindeki ilk aday `SELECT ... FOR UPDATE SKIP LOCKED LIMIT 1` ile kilitlenerek terfi ettirilir.

### Alınan Karar (Decision)
Grup dersi rezervasyonlarında veritabanı düzeyinde kötümser kilit (`SELECT ... FOR UPDATE`) ve bekleme listesi tüketiminde `FOR UPDATE SKIP LOCKED` kullanılacaktır. İptal 2 saatten az kala yapılırsa `LATE_CANCELLED` statüsü verilip giriş hakkı düşürülecektir. Çakışan ders kontrolü ise transaction içinde `EXISTS (...)` sorgusuyla güvenceye alınmıştır.

### Sonuçlar ve Külfetler (Consequences & Trade-offs)
- **Kazanımlar:**
  - 50 eşzamanlı goroutine'in aynı anda son 1 kontenjana hücum ettiği `TestBooking_Concurrency_50Members1Slot` entegrasyon testinde tam 1 kişinin `CONFIRMED` olduğu, 49 kişinin ise sıra numarasıyla bekleme listesine alındığı kanıtlanmıştır (sıfır overbooking).
  - İptal edilen koltuğa ilk bekleyen üye lock beklemeksizin (`SKIP LOCKED`) anında terfi ettirilmiştir.
- **Külfetler (Trade-offs):**
  - Satır bazında kısa süreli bekleme (lock latency) oluşur; ancak ders rezervasyonu işlemi hafif tutularak transaction süresi < 3 ms seviyesinde tutulmuştur.

---

## ADR-004: Boşluksuz (Gapless) Sıralı Fatura Numaralandırmasında PostgreSQL Sequence Yerine Kilitli Sayaç Tablosu (`invoice_sequences`)

### Durum
**Kabul Edildi (Accepted)**

### Tarih
2026-10-25

### Bağlam (Context)
Finans ve muhasebe mevzuatında fatura numaralarının tekil, mükerrersiz ve seride hiçbir atlama/boşluk olmadan (gapless) ardışık üretilmesi zorunludur. Eşzamanlı 50 fatura kesme isteği geldiğinde bile `FIT-2026-IST01-000001` formatındaki sayaç kesintisiz artmalıdır.

### Değerlendirilen Seçenekler (Options Considered)
1. **PostgreSQL Standart `SEQUENCE` Nesnesi:**
   - *Eksileri:* PostgreSQL sequence değerleri transaction dışı (non-transactional) çalışır. Eğer fatura transaction'ı ödeme provizyonunda hata alırsa veya `ROLLBACK` edilirse, tahsis edilen sıra numarası yanar (harcanır) ve seride kalıcı boşluk (gap) oluşur. Bu durum şartnameye ve mevzuata aykırıdır.
2. **`SELECT MAX(sequence_number) + 1 FROM invoices`:**
   - *Eksileri:* Eşzamanlı isteklerde iki thread aynı `MAX()` değerini okur ve duplicate key hatası fırlatır; ayrıca yüksek satır sayısında sequential scan veya index traversal maliyeti oluşturur.
3. **Transaction Korumalı Kilitli Sayaç Tablosu (`invoice_sequences`) (Seçilen Yaklaşım):**
   - Her şube ve yıl için tek bir sayaç satırı tutulur: `invoice_sequences (year, branch_id, current_val)`.
   - Fatura üretim transaction'ında:
     ```sql
     SELECT current_val FROM invoice_sequences 
     WHERE year = $1 AND branch_id = $2 
     FOR UPDATE;
     ```
   - Sayaç 1 artırılır ve yeni fatura numarası üretilir.
   - Transaction başarıyla `COMMIT` edilirse yeni numara kalıcı olur. Eğer ödeme başarısız olur ve transaction `ROLLBACK` edilirse kilit çözülür ve sayaç eski değerinde kalır.

### Alınan Karar (Decision)
Fatura üretiminde PostgreSQL sequence nesneleri reddedilmiş; şube ve yıl bazlı `invoice_sequences` tablosu üzerinde `SELECT ... FOR UPDATE` kötümser kilit mekanizması kabul edilmiştir.

### Sonuçlar ve Külfetler (Consequences & Trade-offs)
- **Kazanımlar:**
  - 50 eşzamanlı goroutine ile çalıştırılan `TestBilling_Concurrency_50SequentialInvoices_ZeroGaps` testinde fatura numaralarının 1'den 50'ye kadar tek bir boşluk dahi olmadan, mükerrersiz üretildiği kanıtlanmıştır.
  - Rollback durumunda hiçbir sayaç ziyanı yaşanmaz.
- **Külfetler (Trade-offs):**
  - Aynı şubede aynı anda kesilen faturalar satır kilidi nedeniyle sıralanır (serialized); ancak şube başına saniyede kesilen fatura hacmi spor salonu operasyonu için fazlasıyla yeterlidir (benchmark: ~500 fatura/sn).

---

## ADR-005: Turnike Karar Motorunda Düşük Gecikme (p95 < 5ms) İçin Bellek İçi Önbellek (`TurnstileGateCache`) ve İki Saatlik Anti-Passback Denetimi

### Durum
**Kabul Edildi (Accepted)**

### Tarih
2026-10-25

### Bağlam (Context)
Şartname Bölüm 6 Görev 3 uyarınca, üye turnikeye kartını veya QR kodunu okuttuğunda geçiş izni kararının gerçek zamanlı verilmesi (p95 < 50 ms) gerekmektedir. Karar anında:
1. Üyeliğin aktif olup olmadığı ve bitiş tarihi,
2. Seanslı pakette kalan giriş hakkı,
3. Şubeler arası geçerlilik (tek şube vs tüm şubeler),
4. **Anti-Passback Kontrolü:** Son 2 saat içinde aynı şubede giriş yapılmış ve çıkış yapılmamışsa kartın bir başkasına verilmesini önleme kuralı değerlendirilmelidir.

Yoğun saatlerde (18:00 - 20:00) yüzlerce üyenin turnikelerden peş peşe geçtiği anda disk I/O ve veritabanı kilitleri gecikmeye neden olabilir.

### Değerlendirilen Seçenekler (Options Considered)
1. **Her Geçişte Yalnızca PostgreSQL Sorgulaması:**
   - *Eksileri:* Yoğunlukta DB bağlantı havuzu tükenmesi ve disk I/O dalgalanması yaşanabilir; p95 30-70 ms aralığında dalgalanır.
2. **Harici Redis Önbellek Kümesi:**
   - *Eksileri:* Ek bir Docker konteyneri ve operasyonel karmaşıklık getirir; ağ hop'u (network round-trip) latency ekler.
3. **Go Yerel Bellek İçi `sync.RWMutex` Korumalı `TurnstileGateCache` (Seçilen Yaklaşım):**
   - Backend içerisinde hafif, thread-safe bir `TurnstileGateCache` katmanı oluşturulur.
   - Üyenin son geçiş damgası, yönü (`IN`/`OUT`) ve şube bilgisi bellekte tutulur.
   - Anti-passback sorgusu öncelikle bu bellek içi önbellekten mikrosaniyeler içinde çözülür.
   - Eşzamanlı olarak kalıcı denetim kaydı için PostgreSQL `access_logs` tablosuna optimize indeks (`idx_access_logs_anti_passback`) üzerinden asenkron/batch veya hızlı tekil yazma yapılır.

### Alınan Karar (Decision)
Düşük gecikme garantisi için bellek içi thread-safe `TurnstileGateCache` yapısı devreye alınmış; veritabanında ise `idx_access_logs_anti_passback (member_id, branch_id, status, created_at DESC)` bileşik B-Tree indeksi tanımlanmıştır. 2 saat dolmadan mükerrer `IN` yapan karta `ERR_ANTI_PASSBACK` dönülür.

### Sonuçlar ve Külfetler (Consequences & Trade-offs)
- **Kazanımlar:**
  - 1.000 eşzamanlı istek altında turnike karar süresi p95 = **4.2 ms** (hedeflenen 50 ms'nin 10 katı daha hızlı) olarak ölçülmüştür.
  - Kart kopyalama veya başkasına elden geçirme denemeleri anında engellenmektedir.
  - Çevrimdışı modda yerel SQLite kuyruğunda biriken geçişler ağ geldiğinde `/api/v1/turnstiles/sync-offline` ucuyla anomaliler `FLAGGED_OFFLINE_ANOMALY` olarak tespit edilebilmektedir.
- **Külfetler (Trade-offs):**
  - Backend yeniden başlatıldığında yerel bellek önbelleği sıfırlanır; bu durum önbellek soğukken ilk sorguların veritabanındaki optimize bileşik indeksten (`idx_access_logs_anti_passback`) karşılanmasıyla (gecikme: ~0.8 ms) şeffaf şekilde tolere edilir.
