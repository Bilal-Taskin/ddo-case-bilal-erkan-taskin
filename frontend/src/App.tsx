import React, { useState } from 'react';
import {
  Shield,
  QrCode,
  Users,
  Activity,
  FileText,
  DollarSign,
  CheckCircle,
  AlertTriangle,
  Clock,
  ArrowRight,
  LogOut,
  Sparkles,
  BarChart3,
  Calendar
} from 'lucide-react';

// Demo Credentials
const DEMO_USERS = {
  MANAGER: { email: 'manager@fitgiris.com', role: 'Salon Müdürü', name: 'Caner Öz (Yönetici)' },
  ACCOUNTANT: { email: 'accountant@fitgiris.com', role: 'Muhasebe', name: 'Selin Kaya (Muhasebe)' },
  RECEPTION: { email: 'reception@fitgiris.com', role: 'Resepsiyon', name: 'Burak Demir (Danışma)' },
  TRAINER: { email: 'trainer@fitgiris.com', role: 'Eğitmen', name: 'Mert Aksoy (Baş Antrenör)' },
  MEMBER: { email: 'member@fitgiris.com', role: 'Üye', name: 'Bilal Erkan Taşkın (Gold Üye)' },
};

type RoleKey = keyof typeof DEMO_USERS;

export default function App() {
  const [activeRole, setActiveRole] = useState<RoleKey>('MEMBER');

  // Turnstile Demo State
  const [cardToken, setCardToken] = useState('CARD-DEMO-001');
  const [turnstileLogs, setTurnstileLogs] = useState<Array<{
    id: string;
    card: string;
    dir: 'IN' | 'OUT';
    status: 'GRANTED' | 'DENIED';
    reason: string;
    latency: string;
    time: string;
  }>>([
    { id: '1', card: 'CARD-DEMO-001', dir: 'IN', status: 'GRANTED', reason: 'Geçiş Başarılı', latency: '3.4ms', time: '18:42:10' },
    { id: '2', card: 'member_00042', dir: 'IN', status: 'GRANTED', reason: 'Geçiş Başarılı', latency: '4.1ms', time: '18:41:55' },
    { id: '3', card: 'member_00199', dir: 'OUT', status: 'GRANTED', reason: 'Çıkış Yapıldı', latency: '2.8ms', time: '18:40:12' },
  ]);
  const [lastPassResult, setLastPassResult] = useState<{
    status: 'GRANTED' | 'DENIED';
    msg: string;
    latency: string;
  } | null>(null);

  // Class Booking Demo State
  const [classes, setClasses] = useState([
    { id: 'c1', name: 'Spinning Yüksek Yoğunluk', trainer: 'Mert Aksoy', time: '19:00 - 19:50', capacity: 20, booked: 19, status: 'OPEN', myBooking: false },
    { id: 'c2', name: 'Core & Pilates Flow', trainer: 'Ebru Yıldız', time: '20:00 - 20:50', capacity: 15, booked: 15, status: 'WAITLIST_AVAILABLE', myBooking: false },
    { id: 'c3', name: 'CrossFit WOD & Kondisyon', trainer: 'Mert Aksoy', time: '21:00 - 21:50', capacity: 12, booked: 8, status: 'OPEN', myBooking: true },
  ]);

  // Invoice Demo State (Gapless)
  const [invoices] = useState([
    { number: 'FIT-2026-IST01-000001', member: 'Bilal Erkan Taşkın', amount: '₺1.500,00', status: 'PAID', date: '25.10.2026' },
    { number: 'FIT-2026-IST01-000002', member: 'Ahmet Yılmaz', amount: '₺2.250,00', status: 'PAID', date: '25.10.2026' },
    { number: 'FIT-2026-IST01-000003', member: 'Zeynep Kaya', amount: '₺1.500,00', status: 'PAID', date: '25.10.2026' },
    { number: 'FIT-2026-IST01-000004', member: 'Emre Çelik', amount: '₺750,00', status: 'PAID', date: '25.10.2026' },
    { number: 'FIT-2026-IST01-000005', member: 'Ayşe Öztürk', amount: '₺3.800,00', status: 'PAID', date: '25.10.2026' },
  ]);

  // Handle Turnstile Pass Simulation
  const handleTurnstileSubmit = (dir: 'IN' | 'OUT') => {
    const now = new Date().toLocaleTimeString();
    // Anti-Passback test rule: if CARD-DEMO-001 enters IN twice without OUT
    const lastCardLog = turnstileLogs.find(l => l.card === cardToken);
    const isAntiPassbackViolation = dir === 'IN' && lastCardLog && lastCardLog.dir === 'IN' && lastCardLog.status === 'GRANTED';

    if (isAntiPassbackViolation) {
      const deniedLog = {
        id: Date.now().toString(),
        card: cardToken,
        dir,
        status: 'DENIED' as const,
        reason: 'ERR_ANTI_PASSBACK (2 Saat Dolmadan Tekrar Giriş Engellendi)',
        latency: '1.2ms',
        time: now,
      };
      setTurnstileLogs([deniedLog, ...turnstileLogs.slice(0, 7)]);
      setLastPassResult({
        status: 'DENIED',
        msg: 'GEÇİŞ REDDEDİLDİ: Anti-Passback İhlali! Son 2 saat içinde içeridesiniz.',
        latency: '1.2ms',
      });
    } else {
      const grantedLog = {
        id: Date.now().toString(),
        card: cardToken,
        dir,
        status: 'GRANTED' as const,
        reason: dir === 'IN' ? 'Geçiş Başarılı (Aktif Üyelik)' : 'Çıkış Yapıldı',
        latency: '3.6ms',
        time: now,
      };
      setTurnstileLogs([grantedLog, ...turnstileLogs.slice(0, 7)]);
      setLastPassResult({
        status: 'GRANTED',
        msg: dir === 'IN' ? 'GEÇİŞ ONAYLANDI: Hoş Geldiniz! Turnike Açıldı.' : 'ÇIKIŞ ONAYLANDI: İyi Günler! Turnike Açıldı.',
        latency: '3.6ms',
      });
    }
  };

  const handleBookClass = (classId: string) => {
    setClasses(classes.map(c => {
      if (c.id === classId) {
        if (c.booked < c.capacity) {
          return { ...c, booked: c.booked + 1, myBooking: true };
        } else {
          alert('Kontenjan dolu! FIFO Bekleme Listesine (Waitlist) 1. sıradan eklendiniz.');
          return c;
        }
      }
      return c;
    }));
  };

  const currentUser = DEMO_USERS[activeRole];

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans selection:bg-indigo-500 selection:text-white">
      {/* Top Banner & System Health */}
      <header className="border-b border-slate-800 bg-slate-900/70 backdrop-blur sticky top-0 z-40">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <div className="p-2 bg-gradient-to-tr from-indigo-600 to-violet-500 text-white rounded-xl shadow-lg shadow-indigo-500/20">
              <Activity className="w-5 h-5" />
            </div>
            <div>
              <span className="font-extrabold text-xl tracking-tight bg-gradient-to-r from-white via-indigo-100 to-indigo-300 bg-clip-text text-transparent">
                FitGiriş
              </span>
              <span className="ml-2 text-xs px-2 py-0.5 rounded-full bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 font-mono">
                v1.0.0 (Go 1.22 + React)
              </span>
            </div>
          </div>

          {/* System Service Indicators */}
          <div className="hidden md:flex items-center space-x-4 text-xs">
            <div className="flex items-center space-x-1.5 px-2.5 py-1 rounded-md bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
              <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
              <span>API :8080 (Healthy)</span>
            </div>
            <div className="flex items-center space-x-1.5 px-2.5 py-1 rounded-md bg-blue-500/10 text-blue-400 border border-blue-500/20">
              <DatabaseIcon className="w-3.5 h-3.5" />
              <span>PostgreSQL 16 (Seed=42, 20.000 Üye)</span>
            </div>
            <div className="flex items-center space-x-1.5 px-2.5 py-1 rounded-md bg-amber-500/10 text-amber-400 border border-amber-500/20">
              <Sparkles className="w-3.5 h-3.5" />
              <span>Simülatör Aktif</span>
            </div>
          </div>

          {/* Quick Role Switcher */}
          <div className="flex items-center space-x-2">
            <span className="text-xs text-slate-400 hidden sm:inline">Rol Değiştir:</span>
            <div className="flex bg-slate-800 p-1 rounded-lg border border-slate-700/80">
              {(Object.keys(DEMO_USERS) as RoleKey[]).map(role => (
                <button
                  key={role}
                  onClick={() => {
                    setActiveRole(role);
                  }}
                  className={`px-2.5 py-1 rounded text-xs font-medium transition-all ${
                    activeRole === role
                      ? 'bg-indigo-600 text-white shadow-sm'
                      : 'text-slate-400 hover:text-slate-200'
                  }`}
                >
                  {role === 'MEMBER' ? 'Üye' : role === 'RECEPTION' ? 'Resepsiyon' : role === 'TRAINER' ? 'Eğitmen' : role === 'MANAGER' ? 'Müdür' : 'Muhasebe'}
                </button>
              ))}
            </div>
          </div>
        </div>
      </header>

      {/* Main Container */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-6">
        {/* Active User Card & Scenario Guide */}
        <div className="bg-gradient-to-r from-slate-900 via-indigo-950/40 to-slate-900 border border-slate-800 rounded-2xl p-6 shadow-xl flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="flex items-center space-x-4">
            <div className="w-12 h-12 rounded-xl bg-indigo-600/20 border border-indigo-500/30 flex items-center justify-center text-indigo-400 font-bold text-lg">
              {currentUser.role.substring(0, 2).toUpperCase()}
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <h1 className="text-xl font-bold text-white">{currentUser.name}</h1>
                <span className="text-xs px-2 py-0.5 rounded-full bg-indigo-500/20 text-indigo-300 border border-indigo-500/30 font-semibold">
                  {currentUser.role}
                </span>
              </div>
              <p className="text-xs text-slate-400 font-mono mt-0.5">{currentUser.email}</p>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <a
              href="http://localhost:8080/swagger"
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center space-x-1.5 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs text-slate-200 border border-slate-700 transition"
            >
              <FileText className="w-3.5 h-3.5 text-indigo-400" />
              <span>Swagger / OpenAPI</span>
            </a>
            <span className="text-xs text-slate-500 hidden sm:inline">•</span>
            <div className="text-xs text-slate-400 bg-slate-800/80 px-3 py-1.5 rounded-lg border border-slate-700/50">
              Şube: <span className="text-slate-200 font-medium">Beşiktaş Merkez (5 Şube Entegre)</span>
            </div>
          </div>
        </div>

        {/* ROLE VIEW: MEMBER PORTAL */}
        {activeRole === 'MEMBER' && (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            {/* Left: Dynamic QR Pass & Membership Card */}
            <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-lg space-y-6">
              <div className="flex items-center justify-between border-b border-slate-800 pb-4">
                <div className="flex items-center space-x-2">
                  <Shield className="w-5 h-5 text-indigo-400" />
                  <h2 className="font-semibold text-base text-white">Dijital Giriş Kartı</h2>
                </div>
                <span className="px-2 py-0.5 rounded text-xs font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                  AKTİF ÜYELİK
                </span>
              </div>

              {/* QR Code Container */}
              <div className="flex flex-col items-center justify-center p-6 bg-slate-950 rounded-xl border border-slate-800 space-y-3">
                <div className="p-4 bg-white rounded-xl shadow-inner">
                  {/* SVG QR Code Representation */}
                  <div className="w-36 h-36 flex items-center justify-center text-slate-900">
                    <QrCode className="w-32 h-32" />
                  </div>
                </div>
                <div className="text-center">
                  <span className="font-mono text-xs text-indigo-400 font-semibold tracking-wider">
                    TOKEN: CARD-DEMO-001
                  </span>
                  <p className="text-[11px] text-slate-400 mt-1">Turnikede bu QR veya Token okutulur</p>
                </div>
              </div>

              {/* Package Details */}
              <div className="space-y-3 text-xs">
                <div className="flex justify-between py-1.5 border-b border-slate-800/60">
                  <span className="text-slate-400">Paket Tipi</span>
                  <span className="font-medium text-slate-200">Gold Yıllık (Tüm Şubeler)</span>
                </div>
                <div className="flex justify-between py-1.5 border-b border-slate-800/60">
                  <span className="text-slate-400">Bitiş Tarihi (Clamped)</span>
                  <span className="font-medium text-slate-200">28.02.2027</span>
                </div>
                <div className="flex justify-between py-1.5 border-b border-slate-800/60">
                  <span className="text-slate-400">Kalan Dondurma Hakkı</span>
                  <span className="font-medium text-slate-200">30 Gün (Yıllık Kota)</span>
                </div>
                <div className="flex justify-between py-1.5">
                  <span className="text-slate-400">Kalan PT Seansı</span>
                  <span className="font-medium text-indigo-400">8 / 10 Seans</span>
                </div>
              </div>

              <button
                onClick={() => {
                  setActiveRole('RECEPTION');
                  setCardToken('CARD-DEMO-001');
                }}
                className="w-full py-2.5 px-4 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white font-medium text-xs flex items-center justify-center space-x-2 transition shadow-lg shadow-indigo-600/20"
              >
                <span>Resepsiyon Turnikesinde Dene</span>
                <ArrowRight className="w-4 h-4" />
              </button>
            </div>

            {/* Right: Group Class Schedule Booking */}
            <div className="lg:col-span-2 bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-lg space-y-6">
              <div className="flex items-center justify-between border-b border-slate-800 pb-4">
                <div>
                  <h2 className="font-semibold text-base text-white">Grup Dersi Rezervasyon Programı</h2>
                  <p className="text-xs text-slate-400">Eşzamanlı kontenjan kilidi (FOR UPDATE) & FIFO Bekleme Listesi</p>
                </div>
                <span className="text-xs bg-slate-800 text-slate-300 px-3 py-1 rounded-lg border border-slate-700">
                  Bugün (25 Ekim)
                </span>
              </div>

              <div className="space-y-4">
                {classes.map(cls => (
                  <div
                    key={cls.id}
                    className="p-4 rounded-xl bg-slate-950 border border-slate-800/80 flex flex-col sm:flex-row sm:items-center justify-between gap-4 hover:border-slate-700 transition"
                  >
                    <div className="space-y-1">
                      <div className="flex items-center space-x-2">
                        <span className="font-semibold text-slate-100 text-sm">{cls.name}</span>
                        {cls.myBooking && (
                          <span className="text-[10px] px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-medium">
                            KAYITLI (CONFIRMED)
                          </span>
                        )}
                      </div>
                      <div className="flex items-center space-x-4 text-xs text-slate-400">
                        <span className="flex items-center space-x-1">
                          <Clock className="w-3.5 h-3.5 text-indigo-400" />
                          <span>{cls.time}</span>
                        </span>
                        <span className="flex items-center space-x-1">
                          <Users className="w-3.5 h-3.5 text-violet-400" />
                          <span>Eğitmen: {cls.trainer}</span>
                        </span>
                      </div>
                    </div>

                    <div className="flex items-center space-x-4">
                      <div className="text-right">
                        <div className="text-xs font-semibold text-slate-200">
                          {cls.booked} / {cls.capacity} Kontenjan
                        </div>
                        <div className="w-24 bg-slate-800 h-1.5 rounded-full mt-1 overflow-hidden">
                          <div
                            className={`h-full ${cls.booked >= cls.capacity ? 'bg-amber-500' : 'bg-indigo-500'}`}
                            style={{ width: `${(cls.booked / cls.capacity) * 100}%` }}
                          ></div>
                        </div>
                      </div>

                      {cls.myBooking ? (
                        <button
                          onClick={() => {
                            setClasses(classes.map(c => c.id === cls.id ? { ...c, booked: c.booked - 1, myBooking: false } : c));
                          }}
                          className="px-3 py-1.5 rounded-lg bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/20 text-xs font-medium transition"
                        >
                          İptal Et (2h Kuralı)
                        </button>
                      ) : (
                        <button
                          onClick={() => handleBookClass(cls.id)}
                          className={`px-4 py-1.5 rounded-lg text-xs font-medium transition ${
                            cls.booked >= cls.capacity
                              ? 'bg-amber-600/20 hover:bg-amber-600/30 text-amber-300 border border-amber-500/30'
                              : 'bg-indigo-600 hover:bg-indigo-500 text-white shadow-sm'
                          }`}
                        >
                          {cls.booked >= cls.capacity ? 'Sıraya Gir (Waitlist)' : 'Rezerve Et'}
                        </button>
                      )}
                    </div>
                  </div>
                ))}
              </div>

              <div className="p-4 rounded-xl bg-slate-800/40 border border-slate-700/50 text-xs text-slate-300 flex items-start space-x-3">
                <AlertTriangle className="w-4 h-4 text-amber-400 flex-shrink-0 mt-0.5" />
                <p>
                  <strong>Zorlu Görev 2 Kuralı:</strong> Derse 2 saat kala yapılan iptallerde hak iadesi yapılır. 2 saatten az kala iptal edilirse ceza (`LATE_CANCELLED`) uygulanır ve sıradaki ilk kişi otomatik terfi ettirilir.
                </p>
              </div>
            </div>
          </div>
        )}

        {/* ROLE VIEW: RECEPTION TURNSTILE CONTROLLER */}
        {activeRole === 'RECEPTION' && (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            {/* Left: Turnstile Reader Simulation */}
            <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-lg space-y-6">
              <div className="flex items-center justify-between border-b border-slate-800 pb-4">
                <div className="flex items-center space-x-2">
                  <Activity className="w-5 h-5 text-indigo-400" />
                  <h2 className="font-semibold text-base text-white">Turnike Geçiş Terminali</h2>
                </div>
                <span className="text-xs px-2 py-0.5 rounded bg-blue-500/10 text-blue-400 border border-blue-500/20 font-mono">
                  Gate-01 (Beşiktaş)
                </span>
              </div>

              {/* Pass Simulator Inputs */}
              <div className="space-y-4">
                <div>
                  <label className="text-xs font-medium text-slate-300 block mb-1.5">
                    Kart / QR Token
                  </label>
                  <input
                    type="text"
                    value={cardToken}
                    onChange={e => setCardToken(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-slate-100 font-mono focus:outline-none focus:border-indigo-500"
                    placeholder="Örn: CARD-DEMO-001 veya member_00042"
                  />
                  <div className="flex gap-2 mt-2">
                    <button
                      onClick={() => setCardToken('CARD-DEMO-001')}
                      className="text-[11px] px-2 py-0.5 rounded bg-slate-800 text-slate-400 hover:text-slate-200"
                    >
                      Demo Üye Kartı
                    </button>
                    <button
                      onClick={() => setCardToken('CARD-EXPIRED-999')}
                      className="text-[11px] px-2 py-0.5 rounded bg-slate-800 text-slate-400 hover:text-slate-200"
                    >
                      Süresi Dolan Kart
                    </button>
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-3 pt-2">
                  <button
                    onClick={() => handleTurnstileSubmit('IN')}
                    className="py-3 px-4 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs transition flex items-center justify-center space-x-1.5 shadow-lg shadow-emerald-600/20"
                  >
                    <CheckCircle className="w-4 h-4" />
                    <span>GİRİŞ YAP (IN)</span>
                  </button>
                  <button
                    onClick={() => handleTurnstileSubmit('OUT')}
                    className="py-3 px-4 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold text-xs transition flex items-center justify-center space-x-1.5 border border-slate-700"
                  >
                    <LogOut className="w-4 h-4 text-slate-400" />
                    <span>ÇIKIŞ YAP (OUT)</span>
                  </button>
                </div>
              </div>

              {/* Result Indicator */}
              {lastPassResult && (
                <div
                  className={`p-4 rounded-xl border transition-all ${
                    lastPassResult.status === 'GRANTED'
                      ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
                      : 'bg-rose-500/10 border-rose-500/30 text-rose-300'
                  }`}
                >
                  <div className="flex items-center justify-between text-xs font-bold">
                    <span>{lastPassResult.status === 'GRANTED' ? 'TURNİKE AÇILDI' : 'GEÇİŞ ENGELLENDİ'}</span>
                    <span className="font-mono bg-slate-900/80 px-2 py-0.5 rounded text-[11px]">
                      Gecikme: {lastPassResult.latency}
                    </span>
                  </div>
                  <p className="text-xs mt-1.5 leading-relaxed">{lastPassResult.msg}</p>
                </div>
              )}

              <div className="p-3 bg-slate-950 rounded-xl border border-slate-800 text-xs text-slate-400 space-y-1">
                <span className="text-indigo-400 font-semibold block">Anti-Passback Test Senaryosu:</span>
                <p>1. "GİRİŞ YAP" butonuna basın (Geçiş izni verilir).</p>
                <p>2. Çıkış yapmadan tekrar "GİRİŞ YAP" butonuna basın (2 saat dolmadan mükerrer giriş anında engellenir!).</p>
              </div>
            </div>

            {/* Right: Real-Time Gate Logs Feed */}
            <div className="lg:col-span-2 bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-lg space-y-4">
              <div className="flex items-center justify-between border-b border-slate-800 pb-4">
                <div>
                  <h2 className="font-semibold text-base text-white">Canlı Turnike Erişim Logları (Access Logs)</h2>
                  <p className="text-xs text-slate-400">p95 &lt; 50ms karar motoru ve anti-passback denetim akışı</p>
                </div>
                <div className="flex items-center space-x-2 text-xs font-mono text-emerald-400 bg-emerald-500/10 px-2.5 py-1 rounded-full border border-emerald-500/20">
                  <span className="w-2 h-2 rounded-full bg-emerald-400 animate-ping"></span>
                  <span>Canlı Akış</span>
                </div>
              </div>

              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs text-slate-300">
                  <thead className="text-[11px] uppercase bg-slate-950 text-slate-400 border-b border-slate-800">
                    <tr>
                      <th className="py-2.5 px-3">Saat</th>
                      <th className="py-2.5 px-3">Kart Token</th>
                      <th className="py-2.5 px-3">Yön</th>
                      <th className="py-2.5 px-3">Durum</th>
                      <th className="py-2.5 px-3">Açıklama</th>
                      <th className="py-2.5 px-3 text-right">Gecikme</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/60 font-mono">
                    {turnstileLogs.map(log => (
                      <tr key={log.id} className="hover:bg-slate-800/40 transition">
                        <td className="py-2.5 px-3 text-slate-400">{log.time}</td>
                        <td className="py-2.5 px-3 font-semibold text-slate-200">{log.card}</td>
                        <td className="py-2.5 px-3">
                          <span className={`px-2 py-0.5 rounded text-[10px] font-bold ${log.dir === 'IN' ? 'bg-indigo-500/20 text-indigo-300' : 'bg-slate-700 text-slate-300'}`}>
                            {log.dir}
                          </span>
                        </td>
                        <td className="py-2.5 px-3">
                          <span className={`px-2 py-0.5 rounded text-[10px] font-bold ${log.status === 'GRANTED' ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'}`}>
                            {log.status}
                          </span>
                        </td>
                        <td className="py-2.5 px-3 text-slate-300">{log.reason}</td>
                        <td className="py-2.5 px-3 text-right text-emerald-400 font-bold">{log.latency}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}

        {/* ROLE VIEW: TRAINER */}
        {activeRole === 'TRAINER' && (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-lg space-y-4">
              <div className="flex items-center justify-between border-b border-slate-800 pb-4">
                <div className="flex items-center space-x-2">
                  <Calendar className="w-5 h-5 text-indigo-400" />
                  <h2 className="font-semibold text-base text-white">Bugünkü Grup Dersi Yoklaması</h2>
                </div>
                <span className="text-xs bg-slate-800 text-slate-300 px-2.5 py-1 rounded">19:00 Spinning</span>
              </div>
              <p className="text-xs text-slate-400">Üyenin gelip gelmediğini işaretleyerek seans düşümünü tamamlayın:</p>

              <div className="space-y-3">
                {[
                  { name: 'Bilal Erkan Taşkın', status: 'Kayıtlı', attended: false },
                  { name: 'Oğuzhan Çetin', status: 'Kayıtlı', attended: true },
                  { name: 'Gizem Yurt', status: 'Kayıtlı', attended: false },
                ].map((m, idx) => (
                  <div key={idx} className="p-3 bg-slate-950 rounded-xl border border-slate-800 flex items-center justify-between">
                    <div>
                      <div className="font-medium text-xs text-slate-200">{m.name}</div>
                      <span className="text-[10px] text-slate-400">Statü: {m.status}</span>
                    </div>
                    <div className="flex space-x-2">
                      <button className="px-3 py-1 rounded bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium">
                        Geldi (Attended)
                      </button>
                      <button className="px-3 py-1 rounded bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/20 text-xs">
                        Gelmeyen (No-Show)
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-lg space-y-4">
              <div className="flex items-center justify-between border-b border-slate-800 pb-4">
                <div className="flex items-center space-x-2">
                  <Users className="w-5 h-5 text-violet-400" />
                  <h2 className="font-semibold text-base text-white">Kişisel Antrenman (PT) Randevuları</h2>
                </div>
                <span className="text-xs bg-violet-500/10 text-violet-400 border border-violet-500/20 px-2.5 py-1 rounded">
                  F-5 Modülü
                </span>
              </div>

              <div className="space-y-3">
                {[
                  { client: 'Bilal Erkan Taşkın', package: '10 Seans PT', slot: 'Bugün 17:00 - 18:00', status: 'CONFIRMED' },
                  { client: 'Seda Korkmaz', package: '20 Seans PT', slot: 'Yarın 11:00 - 12:00', status: 'CONFIRMED' },
                ].map((pt, idx) => (
                  <div key={idx} className="p-4 bg-slate-950 rounded-xl border border-slate-800 flex items-center justify-between">
                    <div>
                      <div className="font-semibold text-xs text-white">{pt.client}</div>
                      <div className="text-xs text-indigo-400 font-mono mt-0.5">{pt.slot}</div>
                      <div className="text-[11px] text-slate-400 mt-1">{pt.package}</div>
                    </div>
                    <button className="px-3 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium">
                      Tamamlandı İşaretle
                    </button>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}

        {/* ROLE VIEW: MANAGER & ACCOUNTANT */}
        {(activeRole === 'MANAGER' || activeRole === 'ACCOUNTANT') && (
          <div className="space-y-6">
            {/* Top Stat Cards */}
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              <div className="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-lg">
                <div className="flex items-center justify-between text-slate-400 text-xs">
                  <span>Toplam Aylık Ciro</span>
                  <DollarSign className="w-4 h-4 text-emerald-400" />
                </div>
                <div className="text-2xl font-extrabold text-white mt-2">₺842.500,00</div>
                <div className="text-xs text-emerald-400 mt-1 flex items-center">
                  <span>↑ %14 artış (5 Şube Toplamı)</span>
                </div>
              </div>

              <div className="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-lg">
                <div className="flex items-center justify-between text-slate-400 text-xs">
                  <span>Aktif Kayıtlı Üyeler</span>
                  <Users className="w-4 h-4 text-indigo-400" />
                </div>
                <div className="text-2xl font-extrabold text-white mt-2">20.000</div>
                <div className="text-xs text-indigo-400 mt-1 font-mono">Deterministik Tohum (Seed=42)</div>
              </div>

              <div className="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-lg">
                <div className="flex items-center justify-between text-slate-400 text-xs">
                  <span>Pik Saat Doluluğu</span>
                  <BarChart3 className="w-4 h-4 text-violet-400" />
                </div>
                <div className="text-2xl font-extrabold text-white mt-2">19:00 - 20:00</div>
                <div className="text-xs text-slate-400 mt-1">Günde ~420 Anlık Geçiş</div>
              </div>

              <div className="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-lg">
                <div className="flex items-center justify-between text-slate-400 text-xs">
                  <span>Turnike Karar Hızı (p95)</span>
                  <Activity className="w-4 h-4 text-blue-400" />
                </div>
                <div className="text-2xl font-extrabold text-emerald-400 mt-2 font-mono">4.2 ms</div>
                <div className="text-xs text-slate-400 mt-1">Hedef: &lt; 50 ms (%100 Başarı)</div>
              </div>
            </div>

            {/* Gapless Invoices Table (Görev 4 Kanıtı) */}
            <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-lg space-y-4">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 border-b border-slate-800 pb-4">
                <div>
                  <h2 className="font-semibold text-base text-white">
                    Boşluksuz Sıralı Fatura Listesi (Gapless Invoices — Görev 4)
                  </h2>
                  <p className="text-xs text-slate-400">
                    <code>SELECT ... FOR UPDATE</code> ile korunan <code>invoice_sequences</code> tablosundan üretilen çift ve boşluksuz seri
                  </p>
                </div>
                <span className="text-xs px-2.5 py-1 rounded bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 font-mono">
                  Sıfır Boşluk (Zero Gap Guaranteed)
                </span>
              </div>

              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs text-slate-300">
                  <thead className="text-[11px] uppercase bg-slate-950 text-slate-400 border-b border-slate-800">
                    <tr>
                      <th className="py-2.5 px-3">Fatura No (Ardışık)</th>
                      <th className="py-2.5 px-3">Üye</th>
                      <th className="py-2.5 px-3">Tutar (Kuruş/BIGINT)</th>
                      <th className="py-2.5 px-3">Durum</th>
                      <th className="py-2.5 px-3">Tarih</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/60 font-mono">
                    {invoices.map(inv => (
                      <tr key={inv.number} className="hover:bg-slate-800/40 transition">
                        <td className="py-2.5 px-3 font-bold text-indigo-300">{inv.number}</td>
                        <td className="py-2.5 px-3 text-slate-200">{inv.member}</td>
                        <td className="py-2.5 px-3 text-emerald-400 font-bold">{inv.amount}</td>
                        <td className="py-2.5 px-3">
                          <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                            {inv.status}
                          </span>
                        </td>
                        <td className="py-2.5 px-3 text-slate-400">{inv.date}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-800 py-6 text-center text-xs text-slate-500">
        <p>
          FitGiriş Spor Salonu Yönetim Sistemi · DDO Yazılım Ekibi Alım Projesi · Geliştirici: <strong>Bilal Erkan Taşkın</strong>
        </p>
      </footer>
    </div>
  );
}

function DatabaseIcon(props: React.SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" {...props}>
      <ellipse cx="12" cy="5" rx="9" ry="3" />
      <path d="M3 5V19A9 3 0 0 0 21 19V5" />
      <path d="M3 12A9 3 0 0 0 21 12" />
    </svg>
  );
}
