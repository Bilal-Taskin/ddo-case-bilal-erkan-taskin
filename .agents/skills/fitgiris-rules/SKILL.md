---
name: fitgiris-rules
description: FitGiriş projesi için zorunlu zeminleme (grounding), şema sabitleme, eşzamanlılık ve veri tipi kuralları.
---

# FitGiriş Proje Kuralları (Grounding & Concurrency Rules)

Bu kurallar FitGiriş projesi geliştirme sürecinde kesinlikle uygulanmalı ve taviz verilmemelidir:

### 1. HER ADIM SONUNDA ZORUNLU KONTROL (Grounding)
Herhangi bir Go kodu yazdığında veya değiştirdiğinde mutlaka terminalde şu komutları çalıştır:
```bash
go build ./...
go test ./...
```
Derleme veya test hatası varsa düzeltmeden adımı bitirme ve asla commit atma.

### 2. ŞEMA SABİTLEME
Veritabanı sorguları (SQL) veya Go struct'ları yazarken tablo ve kolon adlarını asla ezbere uydurma; her zaman `migrations/` klasöründeki `.sql` dosyalarını okuyarak birebir aynı isimleri kullan.

### 3. EŞZAMANLILIK VE VERİ TİPLERİ
- **ORM Kullanımı:** ORM (GORM vb.) kesinlikle yasaktır; doğrudan `jackc/pgx/v5` sürücüsü kullanılacaktır.
- **Para Tutarları:** Tüm para birimi ve tutar alanları `BIGINT` (kuruş cinsinden tamsayı) olarak tutulacaktır.
- **Kilit Mekanizmaları:**
  - Kontenjan yönetimi ve fatura / sipariş sırası işlemleri için: `SELECT ... FOR UPDATE`
  - Bekleme listesi (waiting list / queue) tüketimi için: `SELECT ... FOR UPDATE SKIP LOCKED`

### 4. FAZ ÖZET RAPORU DOKÜMANTASYONU (Phase Documentation)
Her fazın kodlaması, test edilmesi ve commit atılması bittikten sonra, o fazda yapılan tüm teknik geliştirmeleri ve mimari kararları özetleyen bir Markdown dosyasını `docs/phase-{N}-summary.md` adıyla kaydet ve bu raporu da aynı commit'e dahil et.
