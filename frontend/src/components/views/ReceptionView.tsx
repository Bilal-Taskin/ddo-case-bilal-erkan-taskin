import React, { useState, useMemo } from 'react';
import {
  Activity,
  CheckCircle2,
  XCircle,
  LogOut,
  Search,
  Zap,
} from 'lucide-react';
import { ScreenState, TurnstileLog } from '../../types';
import { EmptyState } from '../common/EmptyState';
import { ErrorBanner } from '../common/ErrorBanner';
import { SkeletonCard, SkeletonTable } from '../common/LoadingSkeleton';

interface ReceptionViewProps {
  screenState: ScreenState;
  onSimulateStateChange: (state: ScreenState) => void;
  cardToken: string;
  setCardToken: (token: string) => void;
}

const INITIAL_LOGS: TurnstileLog[] = [
  {
    id: '1',
    card: 'CARD-DEMO-001',
    dir: 'IN',
    status: 'GRANTED',
    reason: 'Geçiş Başarılı (Aktif Gold Üyelik)',
    latency: '3.4ms',
    time: '12:42:10',
    branch: 'Beşiktaş Merkez',
  },
  {
    id: '2',
    card: 'member_00042',
    dir: 'IN',
    status: 'GRANTED',
    reason: 'Geçiş Başarılı (Standart Üye)',
    latency: '4.1ms',
    time: '12:41:55',
    branch: 'Beşiktaş Merkez',
  },
  {
    id: '3',
    card: 'member_00199',
    dir: 'OUT',
    status: 'GRANTED',
    reason: 'Çıkış Yapıldı',
    latency: '2.8ms',
    time: '12:40:12',
    branch: 'Beşiktaş Merkez',
  },
  {
    id: '4',
    card: 'CARD-EXPIRED-999',
    dir: 'IN',
    status: 'DENIED',
    reason: 'ERR_MEMBERSHIP_EXPIRED (Üyelik Süresi Dolmuş)',
    latency: '3.9ms',
    time: '12:38:04',
    branch: 'Beşiktaş Merkez',
  },
];

export const ReceptionView: React.FC<ReceptionViewProps> = ({
  screenState,
  onSimulateStateChange,
  cardToken,
  setCardToken,
}) => {
  const [logs, setLogs] = useState<TurnstileLog[]>(INITIAL_LOGS);
  const [selectedGate, setSelectedGate] = useState('Gate-01 (Turnike A)');
  const [logFilter, setLogFilter] = useState<'ALL' | 'GRANTED' | 'DENIED'>('ALL');
  const [searchCard, setSearchCard] = useState('');
  const [lastPassResult, setLastPassResult] = useState<{
    status: 'GRANTED' | 'DENIED';
    msg: string;
    latency: string;
  } | null>(null);

  const handleTurnstileSubmit = (dir: 'IN' | 'OUT') => {
    const now = new Date().toLocaleTimeString();
    const token = cardToken.trim();

    if (!token) {
      alert('Lütfen bir kart tokenı girin.');
      return;
    }

    if (token === 'CARD-EXPIRED-999') {
      const deniedLog: TurnstileLog = {
        id: Date.now().toString(),
        card: token,
        dir,
        status: 'DENIED',
        reason: 'ERR_MEMBERSHIP_EXPIRED (Paket Süresi Doldu)',
        latency: '3.8ms',
        time: now,
        branch: 'Beşiktaş Merkez',
      };
      setLogs(prev => [deniedLog, ...prev.slice(0, 19)]);
      setLastPassResult({
        status: 'DENIED',
        msg: 'GEÇİŞ ENGELLENDİ: Üyenin paket geçerlilik süresi dolmuştur.',
        latency: '3.8ms',
      });
      return;
    }

    // Anti-Passback rule check: if previous log was IN GRANTED for same card
    const lastCardLog = logs.find(l => l.card === token);
    const isAntiPassbackViolation =
      dir === 'IN' &&
      lastCardLog &&
      lastCardLog.dir === 'IN' &&
      lastCardLog.status === 'GRANTED';

    if (isAntiPassbackViolation) {
      const deniedLog: TurnstileLog = {
        id: Date.now().toString(),
        card: token,
        dir,
        status: 'DENIED',
        reason: 'ERR_ANTI_PASSBACK (2 Saat Dolmadan İkinci Giriş Engellendi)',
        latency: '1.2ms',
        time: now,
        branch: 'Beşiktaş Merkez',
      };
      setLogs(prev => [deniedLog, ...prev.slice(0, 19)]);
      setLastPassResult({
        status: 'DENIED',
        msg: 'GEÇİŞ REDDEDİLDİ: Anti-Passback İhlali! Üye son 2 saat içinde içeride görünüyor.',
        latency: '1.2ms',
      });
    } else {
      const latencyMs = (Math.random() * 2 + 2.5).toFixed(1) + 'ms';
      const grantedLog: TurnstileLog = {
        id: Date.now().toString(),
        card: token,
        dir,
        status: 'GRANTED',
        reason: dir === 'IN' ? 'Geçiş Başarılı (Aktif Üyelik)' : 'Çıkış Yapıldı',
        latency: latencyMs,
        time: now,
        branch: 'Beşiktaş Merkez',
      };
      setLogs(prev => [grantedLog, ...prev.slice(0, 19)]);
      setLastPassResult({
        status: 'GRANTED',
        msg:
          dir === 'IN'
            ? 'GEÇİŞ ONAYLANDI: Hoş Geldiniz! Turnike Kapısı Açıldı.'
            : 'ÇIKIŞ ONAYLANDI: İyi Günler! Turnike Kapısı Açıldı.',
        latency: latencyMs,
      });
    }
  };

  const filteredLogs = useMemo(() => {
    return logs.filter(log => {
      const matchesSearch =
        log.card.toLowerCase().includes(searchCard.toLowerCase()) ||
        log.reason.toLowerCase().includes(searchCard.toLowerCase());
      const matchesFilter =
        logFilter === 'ALL' || log.status === logFilter;
      return matchesSearch && matchesFilter;
    });
  }, [logs, searchCard, logFilter]);

  return (
    <div className="space-y-6">
      {screenState === 'error' && (
        <ErrorBanner
          title="Turnike Servis Bağlantı Hatası"
          message="TurnstileGateCache veya ağ senkronizasyonunda gecikme tespit edildi. Çevrimdışı (offline) moda geçildi."
          onRetry={() => onSimulateStateChange('ready')}
          onDismiss={() => onSimulateStateChange('ready')}
        />
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left: Gate Reader Simulation */}
        <div className="bg-white dark:bg-slate-900 border border-slate-200/80 dark:border-slate-800 rounded-2xl p-5 sm:p-6 shadow-sm hover:shadow-md transition-all space-y-6">
          <div className="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-4">
            <div className="flex items-center space-x-2.5">
              <div className="p-2 rounded-xl bg-brand-50 dark:bg-brand-950/40 text-brand-600 dark:text-brand-400">
                <Activity className="w-5 h-5" />
              </div>
              <div>
                <h2 className="font-bold text-sm sm:text-base text-slate-900 dark:text-white">
                  Turnike Geçiş Terminali
                </h2>
                <span className="text-[11px] text-slate-400 font-mono">
                  p95 &lt; 50ms Karar Motoru
                </span>
              </div>
            </div>
            <span className="px-2.5 py-1 rounded-full text-[11px] font-bold bg-blue-50 text-blue-700 dark:bg-blue-950/60 dark:text-blue-400 border border-blue-200 dark:border-blue-800">
              CANLI
            </span>
          </div>

          {screenState === 'loading' ? (
            <SkeletonCard lines={4} />
          ) : (
            <>
              {/* Gate Selector */}
              <div>
                <label className="text-xs font-semibold text-slate-600 dark:text-slate-300 block mb-1.5">
                  Turnike / Kapı Seçimi
                </label>
                <select
                  value={selectedGate}
                  onChange={e => setSelectedGate(e.target.value)}
                  className="w-full bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 rounded-xl px-3 py-2 text-xs sm:text-sm text-slate-900 dark:text-slate-100 focus:outline-none focus:border-brand-500"
                >
                  <option value="Gate-01 (Turnike A)">Gate-01 (Turnike A - Giriş)</option>
                  <option value="Gate-02 (Turnike B)">Gate-02 (Turnike B - Hızlı Geçiş)</option>
                  <option value="Gate-03 (VIP Turnike)">Gate-03 (VIP Turnike)</option>
                </select>
              </div>

              {/* Card Token Input */}
              <div className="space-y-2">
                <label className="text-xs font-semibold text-slate-600 dark:text-slate-300 block">
                  Kart / QR Token
                </label>
                <input
                  type="text"
                  value={cardToken}
                  onChange={e => setCardToken(e.target.value)}
                  placeholder="Örn: CARD-DEMO-001"
                  className="w-full bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 rounded-xl px-3 py-2 text-xs sm:text-sm text-slate-900 dark:text-slate-100 font-mono focus:outline-none focus:border-brand-500"
                />

                {/* Quick preset buttons */}
                <div className="flex flex-wrap gap-1.5 pt-1">
                  <button
                    onClick={() => setCardToken('CARD-DEMO-001')}
                    className="text-[11px] px-2.5 py-1 rounded-lg bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 hover:bg-brand-50 hover:text-brand-600 dark:hover:bg-slate-700 transition"
                  >
                    Demo Üye
                  </button>
                  <button
                    onClick={() => setCardToken('CARD-EXPIRED-999')}
                    className="text-[11px] px-2.5 py-1 rounded-lg bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 hover:bg-rose-50 hover:text-rose-600 dark:hover:bg-slate-700 transition"
                  >
                    Süresi Dolan Kart
                  </button>
                  <button
                    onClick={() => setCardToken('member_00042')}
                    className="text-[11px] px-2.5 py-1 rounded-lg bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 hover:bg-brand-50 hover:text-brand-600 dark:hover:bg-slate-700 transition"
                  >
                    member_00042
                  </button>
                </div>
              </div>

              {/* Action Buttons */}
              <div className="grid grid-cols-2 gap-3 pt-2">
                <button
                  onClick={() => handleTurnstileSubmit('IN')}
                  className="py-3 px-4 rounded-xl bg-emerald-600 hover:bg-emerald-500 active:bg-emerald-700 text-white font-bold text-xs sm:text-sm transition flex items-center justify-center space-x-2 shadow-sm hover:shadow"
                >
                  <CheckCircle2 className="w-4 h-4" />
                  <span>GİRİŞ (IN)</span>
                </button>
                <button
                  onClick={() => handleTurnstileSubmit('OUT')}
                  className="py-3 px-4 rounded-xl bg-slate-800 hover:bg-slate-700 active:bg-slate-900 text-white font-bold text-xs sm:text-sm transition flex items-center justify-center space-x-2 shadow-sm hover:shadow"
                >
                  <LogOut className="w-4 h-4 text-slate-300" />
                  <span>ÇIKIŞ (OUT)</span>
                </button>
              </div>

              {/* Pass Result Box */}
              {lastPassResult && (
                <div
                  className={`p-4 rounded-2xl border transition-all animate-in fade-in zoom-in-95 duration-200 ${
                    lastPassResult.status === 'GRANTED'
                      ? 'bg-emerald-50 dark:bg-emerald-950/40 border-emerald-200 dark:border-emerald-800 text-emerald-900 dark:text-emerald-100'
                      : 'bg-rose-50 dark:bg-rose-950/40 border-rose-200 dark:border-rose-800 text-rose-900 dark:text-rose-100'
                  }`}
                >
                  <div className="flex items-center justify-between text-xs font-bold mb-1">
                    <span className="flex items-center space-x-1.5">
                      {lastPassResult.status === 'GRANTED' ? (
                        <CheckCircle2 className="w-4 h-4 text-emerald-600 dark:text-emerald-400" />
                      ) : (
                        <XCircle className="w-4 h-4 text-rose-600 dark:text-rose-400" />
                      )}
                      <span>
                        {lastPassResult.status === 'GRANTED'
                          ? 'TURNİKE KAPISI AÇILDI'
                          : 'GEÇİŞ ENGELLENDİ'}
                      </span>
                    </span>
                    <span className="font-mono bg-white dark:bg-slate-900 px-2 py-0.5 rounded text-[11px] shadow-xs">
                      {lastPassResult.latency}
                    </span>
                  </div>
                  <p className="text-xs leading-relaxed opacity-90">{lastPassResult.msg}</p>
                </div>
              )}

              {/* Anti-Passback info */}
              <div className="p-3.5 bg-slate-50 dark:bg-slate-950/80 rounded-xl border border-slate-200/80 dark:border-slate-800 text-xs text-slate-600 dark:text-slate-400 space-y-1">
                <span className="font-semibold text-brand-600 dark:text-brand-400 flex items-center space-x-1">
                  <Zap className="w-3.5 h-3.5" />
                  <span>Anti-Passback Test Adımı:</span>
                </span>
                <p>1. "GİRİŞ (IN)" butonuna basın (Geçiş onaylanır).</p>
                <p>2. Çıkış yapmadan tekrar "GİRİŞ (IN)" butonuna basın (2 saat kuralı engeli tetiklenir).</p>
              </div>
            </>
          )}
        </div>

        {/* Right: Real-Time Gate Logs Feed */}
        <div className="lg:col-span-2 bg-white dark:bg-slate-900 border border-slate-200/80 dark:border-slate-800 rounded-2xl p-5 sm:p-6 shadow-sm hover:shadow-md transition-all space-y-5">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-slate-100 dark:border-slate-800 pb-4">
            <div>
              <div className="flex items-center space-x-2">
                <h2 className="font-bold text-sm sm:text-base text-slate-900 dark:text-white">
                  Canlı Turnike Erişim Logları
                </h2>
                <span className="text-[10px] px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800 font-semibold font-mono">
                  p95: 4.2ms
                </span>
              </div>
              <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                B-Tree indeksli (idx_access_logs_anti_passback) anlık sorgu akışı
              </p>
            </div>
            <div className="flex items-center space-x-2 text-xs font-mono text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-950/60 px-3 py-1.5 rounded-full border border-emerald-200 dark:border-emerald-800">
              <span className="w-2 h-2 rounded-full bg-emerald-500 animate-ping"></span>
              <span>Canlı Akış</span>
            </div>
          </div>

          {/* Search & Filter Bar */}
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
            <div className="relative flex-1">
              <Search className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
              <input
                type="text"
                value={searchCard}
                onChange={e => setSearchCard(e.target.value)}
                placeholder="Kart no veya açıklama ara..."
                className="w-full pl-9 pr-3 py-2 rounded-xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 text-xs sm:text-sm text-slate-900 dark:text-slate-100 placeholder-slate-400 focus:outline-none focus:border-brand-500 transition"
              />
            </div>
            <div className="flex items-center space-x-1.5">
              {(['ALL', 'GRANTED', 'DENIED'] as const).map(f => (
                <button
                  key={f}
                  onClick={() => setLogFilter(f)}
                  className={`px-3 py-1.5 rounded-xl text-xs font-medium transition ${
                    logFilter === f
                      ? 'bg-slate-900 dark:bg-slate-100 text-white dark:text-slate-900 shadow-xs'
                      : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-700'
                  }`}
                >
                  {f === 'ALL' ? 'Tümü' : f === 'GRANTED' ? 'İzin Verildi' : 'Engellendi'}
                </button>
              ))}
            </div>
          </div>

          {/* Logs Table / States */}
          {screenState === 'loading' ? (
            <SkeletonTable rows={5} />
          ) : screenState === 'empty' || filteredLogs.length === 0 ? (
            <EmptyState
              title="Erişim Logu Bulunamadı"
              description="Arama kriterlerinize uyan turnike kaydı bulunamadı. Filtreleri temizleyebilir veya yeni bir geçiş deneyebilirsiniz."
              actionLabel="Tüm Logları Göster"
              onAction={() => {
                setSearchCard('');
                setLogFilter('ALL');
                onSimulateStateChange('ready');
              }}
            />
          ) : (
            <div className="overflow-x-auto rounded-xl border border-slate-200/80 dark:border-slate-800">
              <table className="w-full text-left text-xs text-slate-600 dark:text-slate-300">
                <thead className="text-[11px] uppercase bg-slate-50 dark:bg-slate-950 text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800 font-semibold">
                  <tr>
                    <th className="py-3 px-3.5">Saat</th>
                    <th className="py-3 px-3.5">Kart Token</th>
                    <th className="py-3 px-3.5">Yön</th>
                    <th className="py-3 px-3.5">Durum</th>
                    <th className="py-3 px-3.5">Açıklama</th>
                    <th className="py-3 px-3.5 text-right">Gecikme</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100 dark:divide-slate-800 font-mono">
                  {filteredLogs.map(log => (
                    <tr
                      key={log.id}
                      className="hover:bg-slate-50/80 dark:hover:bg-slate-800/50 transition"
                    >
                      <td className="py-3 px-3.5 text-slate-500 dark:text-slate-400">
                        {log.time}
                      </td>
                      <td className="py-3 px-3.5 font-bold text-slate-900 dark:text-slate-100">
                        {log.card}
                      </td>
                      <td className="py-3 px-3.5">
                        <span
                          className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                            log.dir === 'IN'
                              ? 'bg-brand-50 text-brand-700 dark:bg-brand-950/60 dark:text-brand-300 border border-brand-200 dark:border-brand-800'
                              : 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300'
                          }`}
                        >
                          {log.dir}
                        </span>
                      </td>
                      <td className="py-3 px-3.5">
                        <span
                          className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                            log.status === 'GRANTED'
                              ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800'
                              : 'bg-rose-50 text-rose-700 dark:bg-rose-950/60 dark:text-rose-400 border border-rose-200 dark:border-rose-800'
                          }`}
                        >
                          {log.status}
                        </span>
                      </td>
                      <td className="py-3 px-3.5 text-slate-600 dark:text-slate-300 max-w-xs truncate font-sans">
                        {log.reason}
                      </td>
                      <td className="py-3 px-3.5 text-right font-bold text-emerald-600 dark:text-emerald-400">
                        {log.latency}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
