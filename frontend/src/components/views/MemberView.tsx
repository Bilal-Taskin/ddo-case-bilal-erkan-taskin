import React, { useState, useMemo } from 'react';
import {
  Shield,
  QrCode,
  Users,
  Clock,
  ArrowRight,
  AlertTriangle,
  Search,
  CheckCircle2,
  Copy,
  Check,
  Flame,
  Calendar,
} from 'lucide-react';
import { GymClass, ScreenState } from '../../types';
import { EmptyState } from '../common/EmptyState';
import { ErrorBanner } from '../common/ErrorBanner';
import { SkeletonCard, SkeletonTable } from '../common/LoadingSkeleton';

interface MemberViewProps {
  screenState: ScreenState;
  onSimulateStateChange: (state: ScreenState) => void;
  onNavigateToTurnstile: () => void;
}

const INITIAL_CLASSES: GymClass[] = [
  {
    id: 'c1',
    name: 'Spinning Yüksek Yoğunluk',
    trainer: 'Mert Aksoy',
    time: '19:00 - 19:50',
    capacity: 20,
    booked: 19,
    category: 'Kondisyon',
    intensity: 'Yüksek',
    myBooking: false,
  },
  {
    id: 'c2',
    name: 'Core & Pilates Flow',
    trainer: 'Ebru Yıldız',
    time: '20:00 - 20:50',
    capacity: 15,
    booked: 15,
    category: 'Esneklik',
    intensity: 'Orta',
    myBooking: false,
  },
  {
    id: 'c3',
    name: 'CrossFit WOD & Güç',
    trainer: 'Mert Aksoy',
    time: '21:00 - 21:50',
    capacity: 12,
    booked: 8,
    category: 'Kuvvet',
    intensity: 'Yüksek',
    myBooking: true,
  },
  {
    id: 'c4',
    name: 'Fonksiyonel Antrenman',
    trainer: 'Caner Öz',
    time: '18:00 - 18:45',
    capacity: 16,
    booked: 11,
    category: 'Kuvvet',
    intensity: 'Orta',
    myBooking: false,
  },
];

export const MemberView: React.FC<MemberViewProps> = ({
  screenState,
  onSimulateStateChange,
  onNavigateToTurnstile,
}) => {
  const [classes, setClasses] = useState<GymClass[]>(INITIAL_CLASSES);
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('Tümü');
  const [copiedToken, setCopiedToken] = useState(false);
  const [actionNotice, setActionNotice] = useState<string | null>(null);

  const tokenValue = 'CARD-DEMO-001';

  const copyToClipboard = () => {
    navigator.clipboard?.writeText(tokenValue);
    setCopiedToken(true);
    setTimeout(() => setCopiedToken(false), 2000);
  };

  const handleBook = (cls: GymClass) => {
    if (cls.booked < cls.capacity) {
      setClasses(prev =>
        prev.map(c =>
          c.id === cls.id ? { ...c, booked: c.booked + 1, myBooking: true } : c
        )
      );
      setActionNotice(`"${cls.name}" dersi için rezervasyonunuz başarıyla oluşturuldu.`);
    } else {
      setActionNotice(
        `"${cls.name}" kontenjanı doludur. FIFO Bekleme Listesine (Waitlist) 1. sıradan eklendiniz.`
      );
    }
    setTimeout(() => setActionNotice(null), 4000);
  };

  const handleCancel = (cls: GymClass) => {
    setClasses(prev =>
      prev.map(c =>
        c.id === cls.id ? { ...c, booked: Math.max(0, c.booked - 1), myBooking: false } : c
      )
    );
    setActionNotice(
      `"${cls.name}" rezervasyonu iptal edildi. Bekleme listesindeki ilk aday (SKIP LOCKED) asil kontenjana terfi ettirildi.`
    );
    setTimeout(() => setActionNotice(null), 4000);
  };

  const categories = ['Tümü', 'Kondisyon', 'Kuvvet', 'Esneklik'];

  const filteredClasses = useMemo(() => {
    return classes.filter(cls => {
      const matchesSearch =
        cls.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        cls.trainer.toLowerCase().includes(searchQuery.toLowerCase());
      const matchesCategory =
        selectedCategory === 'Tümü' || cls.category === selectedCategory;
      return matchesSearch && matchesCategory;
    });
  }, [classes, searchQuery, selectedCategory]);

  return (
    <div className="space-y-6">
      {/* Toast Notice */}
      {actionNotice && (
        <div className="p-4 rounded-xl bg-emerald-50 dark:bg-emerald-950/60 border border-emerald-200 dark:border-emerald-800 text-emerald-800 dark:text-emerald-200 text-xs sm:text-sm flex items-center space-x-2.5 shadow-sm animate-in fade-in slide-in-from-top-2 duration-200">
          <CheckCircle2 className="w-5 h-5 text-emerald-600 dark:text-emerald-400 flex-shrink-0" />
          <span className="font-medium">{actionNotice}</span>
        </div>
      )}

      {/* Screen Error State */}
      {screenState === 'error' && (
        <ErrorBanner
          title="Üyelik ve Ders Verileri Alınamadı"
          message="PostgreSQL bağlantısı sırasında geçici bir gecikme oluştu. Lütfen bağlantınızı kontrol edip tekrar deneyin."
          onRetry={() => onSimulateStateChange('ready')}
          onDismiss={() => onSimulateStateChange('ready')}
        />
      )}

      {/* Main Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left: Dynamic QR Pass & Membership Card */}
        <div className="bg-white dark:bg-slate-900 border border-slate-200/80 dark:border-slate-800 rounded-2xl p-5 sm:p-6 shadow-sm hover:shadow-md transition-all space-y-6">
          <div className="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-4">
            <div className="flex items-center space-x-2.5">
              <div className="p-2 rounded-xl bg-brand-50 dark:bg-brand-950/40 text-brand-600 dark:text-brand-400">
                <Shield className="w-5 h-5" />
              </div>
              <div>
                <h2 className="font-bold text-sm sm:text-base text-slate-900 dark:text-white">
                  Dijital Giriş Kartı
                </h2>
                <span className="text-[11px] text-slate-400">Dinamik QR & Token</span>
              </div>
            </div>
            <span className="px-2.5 py-1 rounded-full text-[11px] font-bold bg-emerald-50 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800/80">
              AKTİF
            </span>
          </div>

          {screenState === 'loading' ? (
            <SkeletonCard lines={4} />
          ) : (
            <>
              {/* QR Code Container */}
              <div className="flex flex-col items-center justify-center p-6 bg-slate-50 dark:bg-slate-950/70 rounded-2xl border border-slate-200/70 dark:border-slate-800 space-y-3.5">
                <div className="p-4 bg-white rounded-2xl shadow-sm border border-slate-200/50">
                  <div className="w-36 h-36 flex items-center justify-center text-slate-900">
                    <QrCode className="w-32 h-32 text-slate-900" />
                  </div>
                </div>

                <div className="text-center w-full">
                  <div className="inline-flex items-center space-x-1.5 px-3 py-1.5 bg-white dark:bg-slate-900 rounded-lg border border-slate-200 dark:border-slate-800 font-mono text-xs text-brand-600 dark:text-brand-400 font-bold">
                    <span>{tokenValue}</span>
                    <button
                      onClick={copyToClipboard}
                      className="p-1 hover:text-slate-900 dark:hover:text-white transition"
                      title="Tokenı Kopyala"
                    >
                      {copiedToken ? (
                        <Check className="w-3.5 h-3.5 text-emerald-500" />
                      ) : (
                        <Copy className="w-3.5 h-3.5" />
                      )}
                    </button>
                  </div>
                  <p className="text-[11px] text-slate-400 mt-1.5">
                    Turnikeden geçiş için bu QR veya Tokenı kullanın
                  </p>
                </div>
              </div>

              {/* Package Details */}
              <div className="space-y-2.5 text-xs">
                <div className="flex justify-between py-2 border-b border-slate-100 dark:border-slate-800/60">
                  <span className="text-slate-500 dark:text-slate-400">Paket Adı</span>
                  <span className="font-semibold text-slate-900 dark:text-slate-100">
                    Gold Yıllık (5 Şube Sınırsız)
                  </span>
                </div>
                <div className="flex justify-between py-2 border-b border-slate-100 dark:border-slate-800/60">
                  <span className="text-slate-500 dark:text-slate-400">
                    Bitiş Tarihi (Clamped)
                  </span>
                  <span className="font-semibold text-brand-600 dark:text-brand-400 font-mono">
                    28.02.2027
                  </span>
                </div>
                <div className="flex justify-between py-2 border-b border-slate-100 dark:border-slate-800/60">
                  <span className="text-slate-500 dark:text-slate-400">Dondurma Hakkı</span>
                  <span className="font-semibold text-slate-900 dark:text-slate-100">
                    30 Gün Kalan
                  </span>
                </div>
                <div className="flex justify-between py-2">
                  <span className="text-slate-500 dark:text-slate-400">PT Seansı</span>
                  <span className="font-semibold text-violet-600 dark:text-violet-400">
                    8 / 10 Seans
                  </span>
                </div>
              </div>

              <button
                onClick={onNavigateToTurnstile}
                className="w-full py-3 px-4 rounded-xl bg-brand-600 hover:bg-brand-500 active:bg-brand-700 text-white font-semibold text-xs sm:text-sm flex items-center justify-center space-x-2 transition shadow-sm hover:shadow-md"
              >
                <span>Resepsiyon Turnikesinde Dene</span>
                <ArrowRight className="w-4 h-4" />
              </button>
            </>
          )}
        </div>

        {/* Right: Group Class Schedule Booking */}
        <div className="lg:col-span-2 bg-white dark:bg-slate-900 border border-slate-200/80 dark:border-slate-800 rounded-2xl p-5 sm:p-6 shadow-sm hover:shadow-md transition-all space-y-6">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-slate-100 dark:border-slate-800 pb-4">
            <div>
              <div className="flex items-center space-x-2">
                <h2 className="font-bold text-sm sm:text-base text-slate-900 dark:text-white">
                  Grup Dersi Rezervasyon Programı
                </h2>
                <span className="text-[10px] px-2 py-0.5 rounded-full bg-brand-50 text-brand-700 dark:bg-brand-950/60 dark:text-brand-300 border border-brand-200 dark:border-brand-800 font-semibold">
                  T-2 Modülü
                </span>
              </div>
              <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                Pessimistic Locking (FOR UPDATE) & FIFO Bekleme Listesi (SKIP LOCKED)
              </p>
            </div>
            <div className="flex items-center space-x-2 text-xs text-slate-500 dark:text-slate-400 bg-slate-50 dark:bg-slate-800/80 px-3 py-1.5 rounded-xl border border-slate-200/80 dark:border-slate-700">
              <Calendar className="w-3.5 h-3.5 text-brand-500" />
              <span>Bugün (26 Ekim)</span>
            </div>
          </div>

          {/* Filters & Search Bar */}
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
            <div className="relative flex-1">
              <Search className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
              <input
                type="text"
                value={searchQuery}
                onChange={e => setSearchQuery(e.target.value)}
                placeholder="Ders adı veya eğitmen ara..."
                className="w-full pl-9 pr-3 py-2 rounded-xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 text-xs sm:text-sm text-slate-900 dark:text-slate-100 placeholder-slate-400 focus:outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500 transition"
              />
            </div>
            <div className="flex items-center space-x-1.5 overflow-x-auto pb-1 sm:pb-0">
              {categories.map(cat => (
                <button
                  key={cat}
                  onClick={() => setSelectedCategory(cat)}
                  className={`px-3 py-1.5 rounded-xl text-xs font-medium transition whitespace-nowrap ${
                    selectedCategory === cat
                      ? 'bg-brand-600 text-white shadow-xs'
                      : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-700'
                  }`}
                >
                  {cat}
                </button>
              ))}
            </div>
          </div>

          {/* Class List States */}
          {screenState === 'loading' ? (
            <SkeletonTable rows={4} />
          ) : screenState === 'empty' || filteredClasses.length === 0 ? (
            <EmptyState
              title="Kriterlere Uygun Ders Bulunamadı"
              description="Arama filtrenizi temizleyebilir veya başka bir kategori seçebilirsiniz."
              actionLabel="Filtreleri Temizle"
              onAction={() => {
                setSearchQuery('');
                setSelectedCategory('Tümü');
                onSimulateStateChange('ready');
              }}
            />
          ) : (
            <div className="space-y-3.5">
              {filteredClasses.map(cls => {
                const isFull = cls.booked >= cls.capacity;
                const percent = Math.min(100, Math.round((cls.booked / cls.capacity) * 100));

                return (
                  <div
                    key={cls.id}
                    className="p-4 sm:p-5 rounded-2xl bg-slate-50/70 dark:bg-slate-950/60 border border-slate-200/80 dark:border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-4 hover:border-brand-300 dark:hover:border-slate-700 transition group"
                  >
                    <div className="space-y-1.5 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-bold text-slate-900 dark:text-slate-100 text-sm sm:text-base">
                          {cls.name}
                        </span>
                        <span className="text-[10px] px-2 py-0.5 rounded-md font-semibold bg-slate-200/70 dark:bg-slate-800 text-slate-700 dark:text-slate-300">
                          {cls.category}
                        </span>
                        {cls.myBooking && (
                          <span className="text-[10px] px-2.5 py-0.5 rounded-full bg-emerald-50 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800 font-bold flex items-center space-x-1">
                            <CheckCircle2 className="w-3 h-3 inline mr-1" />
                            KAYITLI (CONFIRMED)
                          </span>
                        )}
                      </div>

                      <div className="flex flex-wrap items-center gap-4 text-xs text-slate-500 dark:text-slate-400">
                        <span className="flex items-center space-x-1.5">
                          <Clock className="w-3.5 h-3.5 text-brand-500" />
                          <span>{cls.time}</span>
                        </span>
                        <span className="flex items-center space-x-1.5">
                          <Users className="w-3.5 h-3.5 text-violet-500" />
                          <span>Eğitmen: {cls.trainer}</span>
                        </span>
                        <span className="flex items-center space-x-1">
                          <Flame className="w-3.5 h-3.5 text-amber-500" />
                          <span>Yoğunluk: {cls.intensity}</span>
                        </span>
                      </div>
                    </div>

                    <div className="flex items-center justify-between sm:justify-end space-x-4 pt-2 sm:pt-0 border-t sm:border-t-0 border-slate-200/50 dark:border-slate-800">
                      <div className="text-left sm:text-right">
                        <div className="text-xs font-bold text-slate-900 dark:text-slate-100 font-mono">
                          {cls.booked} / {cls.capacity} Kontenjan
                        </div>
                        <div className="w-28 bg-slate-200 dark:bg-slate-800 h-2 rounded-full mt-1.5 overflow-hidden">
                          <div
                            className={`h-full transition-all duration-300 ${
                              isFull ? 'bg-amber-500' : 'bg-brand-600'
                            }`}
                            style={{ width: `${percent}%` }}
                          />
                        </div>
                      </div>

                      {cls.myBooking ? (
                        <button
                          onClick={() => handleCancel(cls)}
                          className="px-3.5 py-2 rounded-xl bg-rose-50 hover:bg-rose-100 dark:bg-rose-950/40 dark:hover:bg-rose-900/40 text-rose-700 dark:text-rose-300 border border-rose-200 dark:border-rose-800 text-xs font-semibold transition"
                        >
                          İptal Et (2h Kuralı)
                        </button>
                      ) : (
                        <button
                          onClick={() => handleBook(cls)}
                          className={`px-4 py-2 rounded-xl text-xs font-semibold transition shadow-xs ${
                            isFull
                              ? 'bg-amber-50 hover:bg-amber-100 text-amber-800 dark:bg-amber-950/40 dark:text-amber-300 border border-amber-200 dark:border-amber-800'
                              : 'bg-brand-600 hover:bg-brand-500 active:bg-brand-700 text-white'
                          }`}
                        >
                          {isFull ? 'Sıraya Gir (Waitlist)' : 'Rezerve Et'}
                        </button>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}

          {/* Info Banner */}
          <div className="p-4 rounded-xl bg-amber-50/80 dark:bg-amber-950/30 border border-amber-200 dark:border-amber-800/60 text-xs text-amber-900 dark:text-amber-200 flex items-start space-x-3">
            <AlertTriangle className="w-4 h-4 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5" />
            <p className="leading-relaxed">
              <strong>Zorlu Görev 2 Kuralı:</strong> Derse 2 saatten fazla kala iptal
              edildiğinde ceza uygulanmaz ve bekleme listesindeki ilk aday (FIFO) otomatik
              terfi eder. 2 saatten az kala iptal edilirse ceza (`LATE_CANCELLED`) işlenir.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
};
