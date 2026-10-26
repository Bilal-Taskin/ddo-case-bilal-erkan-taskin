import React from 'react';
import {
  Activity,
  Database,
  Sun,
  Moon,
  ExternalLink,
  Sparkles,
  Menu,
  X,
  LogOut,
} from 'lucide-react';
import { AuthUser, ScreenState } from '../../types';

interface HeaderProps {
  user: AuthUser | null;
  onLogout: () => void;
  isDarkMode: boolean;
  onToggleDarkMode: () => void;
  screenState: ScreenState;
  onStateChange: (state: ScreenState) => void;
  isMobileMenuOpen: boolean;
  onToggleMobileMenu: () => void;
}

export const Header: React.FC<HeaderProps> = ({
  user,
  onLogout,
  isDarkMode,
  onToggleDarkMode,
  screenState,
  onStateChange,
  isMobileMenuOpen,
  onToggleMobileMenu,
}) => {
  const getRoleBadge = (role?: string) => {
    switch (role) {
      case 'MEMBER':
        return { label: 'Üye', bg: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-400 border-emerald-200 dark:border-emerald-800' };
      case 'RECEPTION':
        return { label: 'Resepsiyon', bg: 'bg-blue-50 text-blue-700 dark:bg-blue-950/60 dark:text-blue-400 border-blue-200 dark:border-blue-800' };
      case 'TRAINER':
        return { label: 'Eğitmen', bg: 'bg-amber-50 text-amber-700 dark:bg-amber-950/60 dark:text-amber-400 border-amber-200 dark:border-amber-800' };
      case 'MANAGER':
        return { label: 'Salon Müdürü', bg: 'bg-brand-50 text-brand-700 dark:bg-brand-950/60 dark:text-brand-300 border-brand-200 dark:border-brand-800' };
      case 'ACCOUNTANT':
        return { label: 'Muhasebe', bg: 'bg-violet-50 text-violet-700 dark:bg-violet-950/60 dark:text-violet-300 border-violet-200 dark:border-violet-800' };
      default:
        return { label: role || 'Misafir', bg: 'bg-slate-100 text-slate-700 border-slate-200' };
    }
  };

  const roleInfo = getRoleBadge(user?.role);

  return (
    <header className="sticky top-0 z-40 bg-white/90 dark:bg-slate-900/90 backdrop-blur-md border-b border-slate-200/80 dark:border-slate-800 transition-colors">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between gap-4">
        {/* Sol: Brand Logo */}
        <div className="flex items-center space-x-3 flex-shrink-0">
          <div className="p-2 bg-gradient-to-tr from-brand-600 to-violet-600 text-white rounded-xl shadow-sm shadow-brand-500/20">
            <Activity className="w-5 h-5" />
          </div>
          <div className="flex items-center space-x-2">
            <span className="font-extrabold text-lg sm:text-xl tracking-tight text-slate-900 dark:text-white">
              FitGiriş
            </span>
            <span className="text-[10px] px-2 py-0.5 rounded-full bg-brand-50 text-brand-700 dark:bg-brand-950/80 dark:text-brand-300 border border-brand-200 dark:border-brand-800 font-mono font-bold">
              Enterprise
            </span>
          </div>
        </div>

        {/* Orta: Sistem Linkleri (Temiz, ferah & hafif) */}
        <div className="hidden md:flex items-center space-x-2 text-xs">
          <a
            href="http://localhost:8080/health"
            target="_blank"
            rel="noreferrer"
            className="flex items-center space-x-1.5 px-3 py-1.5 rounded-xl bg-slate-50 dark:bg-slate-800/80 hover:bg-emerald-50 dark:hover:bg-emerald-950/40 text-slate-600 dark:text-slate-300 hover:text-emerald-700 dark:hover:text-emerald-400 border border-slate-200 dark:border-slate-700 transition"
            title="Backend Sağlık Durumu"
          >
            <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
            <span className="font-mono font-medium">API :8080</span>
          </a>

          <a
            href="http://localhost:8085/?pgsql=postgres&username=postgres&db=fitgiris"
            target="_blank"
            rel="noreferrer"
            className="flex items-center space-x-1.5 px-3 py-1.5 rounded-xl bg-slate-50 dark:bg-slate-800/80 hover:bg-blue-50 dark:hover:bg-blue-950/40 text-slate-600 dark:text-slate-300 hover:text-blue-700 dark:hover:text-blue-400 border border-slate-200 dark:border-slate-700 transition"
            title="Adminer Veritabanı Web Arayüzü"
          >
            <Database className="w-3.5 h-3.5 text-blue-500" />
            <span className="font-mono font-medium">Adminer DB</span>
            <ExternalLink className="w-3 h-3 opacity-60" />
          </a>

          <a
            href="http://localhost:8080/swagger"
            target="_blank"
            rel="noreferrer"
            className="flex items-center space-x-1.5 px-3 py-1.5 rounded-xl bg-slate-50 dark:bg-slate-800/80 hover:bg-brand-50 dark:hover:bg-brand-950/40 text-slate-600 dark:text-slate-300 hover:text-brand-700 dark:hover:text-brand-400 border border-slate-200 dark:border-slate-700 transition"
            title="Swagger Canlı API Dokümantasyonu"
          >
            <Sparkles className="w-3.5 h-3.5 text-brand-500" />
            <span className="font-mono font-medium">Swagger</span>
          </a>
        </div>

        {/* Sağ: Durum Dropdown + Rol + Çıkış + Tema */}
        <div className="flex items-center space-x-2 sm:space-x-3 flex-shrink-0">
          {/* Kompakt Durum Seçici (Sıkışıklığı önleyen dropdown) */}
          {user && (
            <div className="hidden sm:flex items-center space-x-1.5 px-2.5 py-1.5 rounded-xl bg-slate-100 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-xs">
              <span className="text-slate-400 text-[11px] font-medium hidden lg:inline">
                Durum:
              </span>
              <select
                value={screenState}
                onChange={e => onStateChange(e.target.value as ScreenState)}
                className="bg-transparent font-semibold text-slate-800 dark:text-slate-200 focus:outline-none cursor-pointer text-xs"
              >
                <option value="ready" className="bg-white dark:bg-slate-900">
                  Normal
                </option>
                <option value="loading" className="bg-white dark:bg-slate-900">
                  Yükleniyor (Loading)
                </option>
                <option value="empty" className="bg-white dark:bg-slate-900">
                  Boş Veri (Empty)
                </option>
                <option value="error" className="bg-white dark:bg-slate-900">
                  Hata (Error)
                </option>
              </select>
            </div>
          )}

          {/* Rol Rozeti */}
          {user && (
            <span
              className={`hidden md:inline-flex text-[11px] font-bold px-2.5 py-1 rounded-full border ${roleInfo.bg}`}
            >
              {roleInfo.label}
            </span>
          )}

          {/* Çıkış Butonu */}
          {user && (
            <button
              onClick={onLogout}
              className="inline-flex items-center space-x-1.5 px-3 py-1.5 rounded-xl bg-slate-100 hover:bg-rose-50 dark:bg-slate-800 dark:hover:bg-rose-950/40 text-slate-700 hover:text-rose-600 dark:text-slate-300 dark:hover:text-rose-400 border border-slate-200 hover:border-rose-200 dark:border-slate-700 dark:hover:border-rose-800 text-xs font-semibold transition"
              title="Oturumu Kapat"
            >
              <LogOut className="w-3.5 h-3.5" />
              <span className="hidden sm:inline">Çıkış</span>
            </button>
          )}

          {/* Tema Değiştirici */}
          <button
            onClick={onToggleDarkMode}
            aria-label="Tema Değiştir"
            className="p-2 rounded-xl bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 border border-slate-200 dark:border-slate-700 transition"
            title={isDarkMode ? 'Açık Temaya Geç' : 'Koyu Temaya Geç'}
          >
            {isDarkMode ? (
              <Sun className="w-4 h-4 text-amber-400" />
            ) : (
              <Moon className="w-4 h-4 text-slate-600" />
            )}
          </button>

          {/* Mobil Menü Butonu */}
          <button
            onClick={onToggleMobileMenu}
            className="md:hidden p-2 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 border border-slate-200 dark:border-slate-700"
          >
            {isMobileMenuOpen ? (
              <X className="w-5 h-5" />
            ) : (
              <Menu className="w-5 h-5" />
            )}
          </button>
        </div>
      </div>

      {/* Mobil Çekmece */}
      {isMobileMenuOpen && (
        <div className="md:hidden border-t border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-4 space-y-3 animate-in slide-in-from-top-2 duration-200">
          {user && (
            <div className="flex items-center justify-between pb-3 border-b border-slate-100 dark:border-slate-800">
              <div>
                <div className="font-bold text-sm text-slate-900 dark:text-white">
                  {user.first_name} {user.last_name}
                </div>
                <div className="text-xs text-slate-400 font-mono">{user.email}</div>
              </div>
              <button
                onClick={onLogout}
                className="px-3 py-1.5 rounded-lg bg-rose-50 text-rose-600 dark:bg-rose-950/50 dark:text-rose-400 text-xs font-semibold"
              >
                Çıkış Yap
              </button>
            </div>
          )}

          {user && (
            <div className="space-y-1.5">
              <span className="text-[11px] font-bold text-slate-400 uppercase tracking-wider block">
                Ekran Durumu Simülatörü
              </span>
              <div className="grid grid-cols-4 gap-1.5 text-xs">
                {(['ready', 'loading', 'empty', 'error'] as const).map(st => (
                  <button
                    key={st}
                    onClick={() => onStateChange(st)}
                    className={`py-1.5 rounded-lg text-center font-medium ${
                      screenState === st
                        ? 'bg-slate-900 text-white dark:bg-white dark:text-slate-900 font-bold'
                        : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300'
                    }`}
                  >
                    {st === 'ready'
                      ? 'Normal'
                      : st === 'loading'
                      ? 'Yükleniyor'
                      : st === 'empty'
                      ? 'Boş Veri'
                      : 'Hata'}
                  </button>
                ))}
              </div>
            </div>
          )}

          <div className="pt-2 border-t border-slate-100 dark:border-slate-800 flex flex-wrap gap-2 text-xs">
            <a
              href="http://localhost:8085/?pgsql=postgres&username=postgres&db=fitgiris"
              target="_blank"
              rel="noreferrer"
              className="flex items-center space-x-1.5 px-3 py-1.5 rounded-lg bg-blue-50 dark:bg-blue-950/50 text-blue-700 dark:text-blue-400 border border-blue-200 dark:border-blue-800"
            >
              <Database className="w-3.5 h-3.5" />
              <span>Adminer DB</span>
            </a>
            <a
              href="http://localhost:8080/swagger"
              target="_blank"
              rel="noreferrer"
              className="flex items-center space-x-1.5 px-3 py-1.5 rounded-lg bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-700"
            >
              <Sparkles className="w-3.5 h-3.5 text-brand-500" />
              <span>Swagger API</span>
            </a>
          </div>
        </div>
      )}
    </header>
  );
};
