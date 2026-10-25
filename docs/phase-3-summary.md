# Faz 3 Özet Raporu: Üyelik Yaşam Döngüsü, Clamped Date Aritmetiği ve Durum Makinesi (FSM)

## 1. Genel Bakış
Faz 3 kapsamında şartnamede belirtilen **Görev 1 (T-1)** ve **F-1 (Üyelik paketleri ve satış)** tamamlanmıştır. Spor salonu üyelik paketleri, ay sonu artık gün taşmalarını (`month-overflow`) önleyen özel Clamped Date aritmetiği, geçersiz geçişleri engelleyen Durum Makinesi (FSM), dondurma (freeze), devir (transfer), orantılı yükseltme (prorated upgrade) ve iptal kuralları kodlanmış ve doğrulanmıştır.

## 2. Yapılan Teknik Geliştirmeler

### A. Clamped Date Aritmetiği (`internal/platform/dateutil`)
- **Sorun:** Go standart kütüphanesindeki `time.AddDate(0, 1, 0)` fonksiyonu, 31 Ocak gibi aylara 1 ay eklendiğinde takvimde var olmayan 31 Şubat yerine Mart ayına (artık yıla göre 2 Mart veya 3 Mart) taşmaktadır.
- **Çözüm:** `AddClampedMonths(t time.Time, months int) time.Time` fonksiyonu geliştirildi. Artık yıl kontrolüyle hedef ayın maksimum gün sayısı (`daysInMonth`) tespit edilerek kaynak gün hedef ayın son gününe sabitlenmektedir:
  - `2024-01-31` + 1 ay $\rightarrow$ `2024-02-29` (artık yıl)
  - `2025-01-31` + 1 ay $\rightarrow$ `2025-02-28` (normal yıl)
  - `2024-02-29` + 12 ay $\rightarrow$ `2025-02-28`
  - 31 Mart, 31 Mayıs, 31 Ağustos, 31 Ekim gibi 30 çeken aylarda gün 30'a sabitlenir.

### B. Üyelik Durum Makinesi (FSM) (`internal/domain/membership.go`)
- **Durumlar:** `PENDING_PAYMENT`, `ACTIVE`, `FROZEN`, `TRANSFERRED`, `SUSPENDED`, `EXPIRED`, `CANCELLED`.
- **Geçiş Matrisi:**
  - `PENDING_PAYMENT` $\rightarrow$ `ACTIVE`, `CANCELLED`
  - `ACTIVE` $\rightarrow$ `FROZEN`, `TRANSFERRED`, `SUSPENDED`, `EXPIRED`, `CANCELLED`
  - `FROZEN` $\rightarrow$ `ACTIVE`, `CANCELLED`, `EXPIRED`
  - `SUSPENDED` $\rightarrow$ `ACTIVE`, `CANCELLED`, `EXPIRED`
  - `TRANSFERRED`, `EXPIRED`, `CANCELLED` terminal (nihai) durumlardır; hiçbir duruma geçemezler.
- Geçersiz geçiş teşebbüslerinde HTTP `409 Conflict` ve `ERR_INVALID_STATE_TRANSITION` kodu döndürülür.

### C. Dondurma ve Erken Çözülme Kuralları
- Her paketin `max_freeze_days` (varsayılan 30 gün) üst sınırı transaction içinde kontrol edilir (`alreadyFrozen + requestedDays <= max_freeze_days`).
- Dondurma süresi kadar bitiş tarihi (`end_date`) ileri ötelenir.
- Erken çözülme (`unfreeze`) durumunda fiilen kullanılmayan günler hesaplanır, bitiş tarihinden düşülerek üyenin dondurma hakkı iade edilir.

### D. Devir (Transfer) Kuralları
- Kaynak üyelik `TRANSFERRED` durumuna alınır; hedef üyeye kalan gün veya seans hakkı kadar yeni aktif üyelik açılır (`package_transfers` tablosuna kaydedilir).
- Süresi dolmuş paketlerin veya üyenin kendisine devir yapması engellenir.

### E. Orantılı Yükseltme (Prorated Upgrade)
- Formül:
  $$\text{Kalan Değer} = \left\lfloor \frac{\text{Kalan Gün}}{\text{Toplam Gün}} \times \text{Mevcut Paket Fiyatı (kuruş)} \right\rfloor$$
  $$\text{Ödenecek Fark} = \max\left(0, \text{Yeni Paket Fiyatı} - \text{Kalan Değer}\right)$$
- `GetUpgradeQuote` ile önceden hesaplanan fark sunulur; onaylandığında eski üyelik kapatılıp yeni paket başlatılır.

### F. HTTP Uç Noktaları ve Eşzamanlılık Kilidi
- Pessimistic Row Lock (`SELECT ... FOR UPDATE OF mp`) uygulanarak yarış durumları (race condition) engellendi.
- Uçlar:
  - `GET /api/v1/packages`, `POST /api/v1/packages`, `GET /api/v1/packages/{id}`
  - `POST /api/v1/memberships/purchase`
  - `GET /api/v1/memberships`, `GET /api/v1/memberships/{id}`
  - `POST /api/v1/memberships/{id}/freeze`, `POST /api/v1/memberships/{id}/unfreeze`
  - `POST /api/v1/memberships/{id}/transfer`
  - `GET /api/v1/memberships/{id}/upgrade-quote`, `POST /api/v1/memberships/{id}/upgrade`
  - `POST /api/v1/memberships/{id}/cancel`

## 3. Doğrulama ve Test Sonuçları
- `dateutil_test.go`: Ay sonu taşmaları, artık yıl, negatif ay eklemeleri doğrulandı.
- `membership_test.go`: FSM durum geçişleri ve kısıtları test edildi.
- `membership_lifecycle_test.go`: Satın alma $\rightarrow$ dondurma $\rightarrow$ erken çözme $\rightarrow$ transfer $\rightarrow$ iptal döngüsü uçtan uca test edildi.
- `membership_handler_test.go`: HTTP durum kodları (200, 201, 401, 403, 409 Conflict) doğrulandı.
- Tüm `go test ./...` ve `go build ./...` testleri sıfır hata ile geçti.
