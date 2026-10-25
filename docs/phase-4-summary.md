# Faz 4 Özet Raporu: Grup Dersi Rezervasyonu, Kapasite Yönetimi, Bekleme Listesi ve Eşzamanlılık Kilidi (T-2 & F-4)

## 1. Genel Bakış
Faz 4 kapsamında `proje.md` şartnamesinde belirtilen **Görev 2 (T-2)** ve **F-4 (Grup dersi programı ve rezervasyon)** özellikleri eksiksiz olarak kodlanmış, test edilmiş ve mimari gereksinimler yerine getirilmiştir. 

Sistemde:
- 50 kişinin son 1 kontenjana aynı anda saldırdığı yüksek eşzamanlılık (concurrency) senaryosu `SELECT ... FOR UPDATE` pessimist satır kilidi ile çözülmüş ve yarış durumu (race condition) sıfıra indirilmiştir.
- Kapasite dolduğunda üyeleri FIFO sırasıyla sıraya alan `class_waitlist` bekleme listesi entegre edilmiştir.
- 2 saat iptal kuralı ve geç iptal cezası (2 saatten az kala iptalde `LATE_CANCELLED`, `penalty_applied = true`, giriş hakkı yanması) uygulanmıştır.
- Bir üye iptal yaptığında boşalan kontenjan için bekleme listesindeki ilk üye `SELECT ... FOR UPDATE SKIP LOCKED` mekanizması ile kilitlenip otomatik olarak `CONFIRMED` statüsüne terfi ettirilmektedir.
- Aynı üyenin çakışan saatteki başka bir derse rezervasyon yapması engellenmiştir.
- Eğitmen/yetkili yoklama (`ATTENDED` / `NO_SHOW`) ve no-show cezası altyapısı kurulmuştur.

---

## 2. Yapılan Teknik Geliştirmeler ve Mimari Kararlar

### A. Eşzamanlılık ve Pessimist Kilitleme (`SELECT ... FOR UPDATE`)
- **Problem:** Aynı anda 50 kişi son 1 kontenjanlı derse rezervasyon yapmaya çalıştığında, `SELECT booked_count` ve `UPDATE` arasındaki yarış durumundan ötürü birden fazla kişi koltuk kapabilmekte veya `booked_count > capacity` CHECK constraint ihlali oluşabilmektedir.
- **Çözüm:** Rezervasyon işlemi sırasında `class_schedules` tablosu pessimist satır kilidi ile kilitlenir:
  ```sql
  SELECT s.id, s.class_id, s.branch_id, s.trainer_id, s.start_time, s.end_time,
         s.capacity, s.booked_count, s.status, s.created_at, s.updated_at
  FROM class_schedules s
  WHERE s.id = $1
  FOR UPDATE OF s;
  ```
- Kilit sayesinde gelen tüm eşzamanlı istekler sıraya alınır:
  - İlk istek `booked_count < capacity` görür, `booked_count` değerini 1 artırır ve `CONFIRMED` rezervasyonu oluşturur.
  - Ardından gelen sonraki tüm istekler `booked_count >= capacity` görür ve otomatik olarak `class_waitlist` tablosuna `WAITING` statüsünde ve FIFO sırasıyla (`created_at`) eklenir.

### B. Bekleme Listesi Tüketimi (`SELECT ... FOR UPDATE SKIP LOCKED`)
- **Kural:** Şartname ve `fitgiris-rules` kuralı gereğince kuyruk/bekleme listesi tüketimi için:
  ```sql
  SELECT id, schedule_id, member_id, status, created_at, promoted_at
  FROM class_waitlist
  WHERE schedule_id = $1 AND status = 'WAITING'
  ORDER BY created_at ASC
  LIMIT 1
  FOR UPDATE SKIP LOCKED;
  ```
- Bu sorgu ile ilk bekleyen kişi diğer eşzamanlı işlemler tarafından bloke edilmeden/bekletilmeden kilitlenir, `PROMOTED` yapılır ve anında `CONFIRMED` rezervasyonu açılır.

### C. 2 Saat İptal Kuralı ve Ceza Mekanizması
- **Zaman Kontrolü:** `domain.IsLateCancellation(startTime, now)`:
  - $\Delta t \ge 2\text{ saat}$: **Ücretsiz İptal.** Rezervasyon `CANCELLED` yapılır, `penalty_applied = false`. Üyenin paketi seanslı (`SESSION`) ise 1 giriş hakkı iade edilir (`remaining_entries + 1`).
  - $\Delta t < 2\text{ saat}$: **Geç İptal Cezası.** Rezervasyon `LATE_CANCELLED` yapılır, `penalty_applied = true`. Seans hakkı iade **edilmez** (yanar).
- İptal sonrasında bekleme listesinde bekleyen varsa ilk bekleyen terfi ettirilir ve `booked_count` sabit kalır; bekleyen yoksa `booked_count` 1 azaltılır.

### D. Çakışan Ders Kontrolü (Overlapping Schedule Prevention)
- Rezervasyon sırasında üyenin aynı zaman diliminde (`s.start_time < new_end_time AND s.end_time > new_start_time`) başka bir aktif (`CONFIRMED`) dersi olup olmadığı transaction içinde kontrol edilir:
  - Çakışma varsa HTTP `409 Conflict` ve `ERR_TIME_CONFLICT` ile reddedilir.

### E. Üyelik ve Şube Yetki Kontrolleri
- Rezervasyon yapılabilmesi için üyenin geçerli ve süresi dolmamış bir `ACTIVE` üyeliği bulunmalıdır (`ErrNoActiveMembership`).
- Tek şubeli üyeliklerde dersin açıldığı şube ile üyenin şubesinin uyuşması şarttır (`ErrBranchAccessDenied`).
- Seanslı paketlerde (`SESSION`) `remaining_entries > 0` olmalıdır (`ErrInsufficientEntries`).

---

## 3. Eklenen Dosyalar ve Katmanlar

1. `backend/internal/domain/booking.go`:
   - `Class`, `ClassSchedule`, `ClassBooking`, `ClassWaitlist` modelleri.
   - Durum sabitleri: `ScheduleStatus`, `BookingStatus`, `WaitlistStatus`.
   - `IsLateCancellation(startTime, cancelTime)` fonksiyonu ve domain hataları.
2. `backend/internal/domain/booking_test.go`:
   - 2 saat kuralı sınır değer (boundary) testleri ve durum doğrulama testleri.
3. `backend/internal/repository/booking_repo.go`:
   - Ham `pgx/v5` sorguları, transaction desteği, `FOR UPDATE` ve `FOR UPDATE SKIP LOCKED` implementasyonları.
4. `backend/internal/service/booking_service.go`:
   - `BookClass`, `CancelBooking`, `CancelWaitlist`, `MarkAttendance`, `CreateClass`, `CreateSchedule` iş mantıkları.
5. `backend/internal/service/booking_concurrency_test.go`:
   - 50 kişinin son 1 kontenjana saldırdığı yarış durumu ve eşzamanlı çoklu iptal terfi testleri.
6. `backend/internal/service/booking_service_test.go`:
   - İptal kuralları, iade/ceza senaryoları, yoklama ve zaman çakışması birim testleri.
7. `backend/internal/handler/booking_handler.go`:
   - HTTP REST uçları:
     - `GET /api/v1/classes`
     - `POST /api/v1/classes` (MANAGER)
     - `GET /api/v1/classes/schedules`
     - `POST /api/v1/classes/schedules` (MANAGER, TRAINER)
     - `GET /api/v1/classes/schedules/{id}`
     - `POST /api/v1/classes/schedules/{id}/book` (MEMBER)
     - `POST /api/v1/classes/bookings/{id}/cancel` (MEMBER)
     - `POST /api/v1/classes/waitlist/{id}/cancel` (MEMBER)
     - `GET /api/v1/classes/my-bookings` (MEMBER)
     - `GET /api/v1/classes/my-waitlist` (MEMBER)
     - `POST /api/v1/classes/schedules/{id}/attendance` (TRAINER, RECEPTION, MANAGER)
8. `backend/internal/handler/booking_handler_test.go`:
   - HTTP uçlarının RBAC yetkilendirme, durum kodları (201, 202, 401, 403, 409) testleri.

---

## 4. Test ve Doğrulama Sonuçları

- **Eşzamanlılık Testi (`TestConcurrency_50MembersCompeteForLast1Spot`):**
  - 50 goroutine aynı mikro-saniyede rezervasyon isteği göndermiştir.
  - Sonuç: Tam olarak **1 CONFIRMED** rezervasyon yapılmış, diğer **49 üye sırasıyla bekleme listesine (WAITING)** alınmıştır.
  - Kontenjan aşımı olmamış, `booked_count` strictly 1 olarak kalmıştır.
  - 1. üyenin iptaliyle, bekleme listesindeki 1. sıradaki üye anında ve otomatik olarak `CONFIRMED` statüsüne terfi etmiştir.
- **Tüm Testler:**
  - `go test -count=1 ./...` komutu 98 testin tamamında başarıyla geçmiştir (`PASS`).
