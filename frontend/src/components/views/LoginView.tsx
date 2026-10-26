import React, { useState } from 'react';
import {
  Activity,
  Lock,
  Mail,
  ArrowRight,
  Shield,
  Building2,
  Calendar,
  CreditCard,
  Eye,
  EyeOff,
  Loader2,
  CheckCircle2,
} from 'lucide-react';
import { AuthUser, AuthTokens, RoleKey } from '../../types';
import { ErrorBanner } from '../common/ErrorBanner';

interface LoginViewProps {
  onLoginSuccess: (user: AuthUser, tokens: AuthTokens) => void;
}

const DEMO_PRESETS: Array<{
  role: RoleKey;
  label: string;
  email: string;
  name: string;
  icon: React.ReactNode;
  desc: string;
}> = [
  {
    role: 'MEMBER',
    label: 'Üye',
    email: 'member@fitgiris.com',
    name: 'Bilal Erkan Taşkın',
    icon: <Shield className="w-4 h-4 text-emerald-500" />,
    desc: 'QR geçiş, paket durumu ve ders rezervasyonu',
  },
  {
    role: 'RECEPTION',
    label: 'Resepsiyon',
    email: 'reception@fitgiris.com',
    name: 'Burak Demir',
    icon: <Activity className="w-4 h-4 text-blue-500" />,
    desc: 'Turnike kapı kontrolü ve anti-passback logları',
  },
  {
    role: 'TRAINER',
    label: 'Eğitmen',
    email: 'trainer@fitgiris.com',
    name: 'Mert Aksoy',
    icon: <Calendar className="w-4 h-4 text-amber-500" />,
    desc: 'Grup dersi yoklaması ve PT randevuları',
  },
  {
    role: 'MANAGER',
    label: 'Salon Müdürü',
    email: 'manager@fitgiris.com',
    name: 'Caner Öz',
    icon: <Building2 className="w-4 h-4 text-brand-500" />,
    desc: '5 şube analitiği, doluluk ve ciro raporları',
  },
  {
    role: 'ACCOUNTANT',
    label: 'Muhasebe',
    email: 'accountant@fitgiris.com',
    name: 'Selin Kaya',
    icon: <CreditCard className="w-4 h-4 text-violet-500" />,
    desc: 'Boşluksuz sıralı faturalar ve tahsilat takibi',
  },
];

export const LoginView: React.FC<LoginViewProps> = ({ onLoginSuccess }) => {
  const [email, setEmail] = useState('member@fitgiris.com');
  const [password, setPassword] = useState('Password123!');
  const [showPassword, setShowPassword] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const handleSubmit = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    setErrorMessage(null);
    setIsLoading(true);

    try {
      // Backend API login endpoint call
      const res = await fetch('http://localhost:8080/api/v1/auth/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email: email.trim(), password }),
      });

      const json = await res.json();

      if (!res.ok) {
        throw new Error(
          json?.error?.message ||
            json?.message ||
            'Giriş başarısız. Lütfen e-posta ve parolanızı kontrol edin.'
        );
      }

      const userData: AuthUser = {
        id: json.data.user.id,
        email: json.data.user.email,
        first_name: json.data.user.first_name,
        last_name: json.data.user.last_name,
        role: json.data.user.role as RoleKey,
        home_branch_id: json.data.user.home_branch_id,
      };

      const tokensData: AuthTokens = {
        access_token: json.data.tokens.access_token,
        refresh_token: json.data.tokens.refresh_token,
        access_expiry: json.data.tokens.access_expiry,
      };

      onLoginSuccess(userData, tokensData);
    } catch (err: unknown) {
      if (err instanceof Error) {
        setErrorMessage(err.message);
      } else {
        setErrorMessage('Sunucuya bağlanırken bir hata oluştu.');
      }
    } finally {
      setIsLoading(false);
    }
  };

  const handleSelectPreset = (presetEmail: string) => {
    setEmail(presetEmail);
    setPassword('Password123!');
    setErrorMessage(null);
  };

  return (
    <div className="min-h-screen flex flex-col justify-center py-12 sm:px-6 lg:px-8 bg-slate-50/70 dark:bg-slate-950 transition-colors">
      <div className="sm:mx-auto sm:w-full sm:max-w-md text-center">
        {/* Brand Icon */}
        <div className="inline-flex p-3 rounded-2xl bg-gradient-to-tr from-brand-600 to-violet-600 text-white shadow-lg shadow-brand-500/20 mb-3">
          <Activity className="w-8 h-8" />
        </div>
        <h1 className="text-2xl sm:text-3xl font-extrabold text-slate-900 dark:text-white tracking-tight">
          FitGiriş Platformu
        </h1>
        <p className="mt-1 text-xs sm:text-sm text-slate-500 dark:text-slate-400">
          Rol Tabanlı Güvenli Oturum Açma (RBAC)
        </p>
      </div>

      <div className="mt-6 sm:mx-auto sm:w-full sm:max-w-xl px-4 sm:px-0">
        <div className="bg-white dark:bg-slate-900 py-8 px-6 sm:px-10 shadow-sm hover:shadow-md border border-slate-200/80 dark:border-slate-800 rounded-3xl transition-all space-y-6">
          {/* Error Banner */}
          {errorMessage && (
            <ErrorBanner
              title="Kimlik Doğrulama Hatası"
              message={errorMessage}
              onDismiss={() => setErrorMessage(null)}
            />
          )}

          {/* Login Form */}
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1">
                E-Posta Adresi
              </label>
              <div className="relative">
                <Mail className="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
                <input
                  type="email"
                  required
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  placeholder="ornek@fitgiris.com"
                  className="w-full pl-10 pr-3.5 py-2.5 rounded-xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 text-xs sm:text-sm text-slate-900 dark:text-slate-100 placeholder-slate-400 focus:outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500 font-mono transition"
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1">
                Parola
              </label>
              <div className="relative">
                <Lock className="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
                <input
                  type={showPassword ? 'text' : 'password'}
                  required
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                  placeholder="••••••••••••"
                  className="w-full pl-10 pr-10 py-2.5 rounded-xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 text-xs sm:text-sm text-slate-900 dark:text-slate-100 placeholder-slate-400 focus:outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500 font-mono transition"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition"
                >
                  {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              </div>
            </div>

            <button
              type="submit"
              disabled={isLoading}
              className="w-full py-3 px-4 rounded-xl bg-brand-600 hover:bg-brand-500 active:bg-brand-700 disabled:opacity-50 text-white font-bold text-xs sm:text-sm transition flex items-center justify-center space-x-2 shadow-sm hover:shadow"
            >
              {isLoading ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  <span>Kimlik Doğrulanıyor...</span>
                </>
              ) : (
                <>
                  <span>Giriş Yap</span>
                  <ArrowRight className="w-4 h-4" />
                </>
              )}
            </button>
          </form>

          {/* Quick Demo Selector for Reviewers */}
          <div className="pt-4 border-t border-slate-100 dark:border-slate-800 space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-xs font-bold text-slate-700 dark:text-slate-300">
                Hızlı Rol Seçimi (Jüri & Test Kolaylığı)
              </span>
              <span className="text-[11px] text-slate-400 font-mono">
                Parola: Password123!
              </span>
            </div>
            <p className="text-[11px] text-slate-500 dark:text-slate-400 leading-relaxed">
              Şartname Bölüm 4 gereğince, her rol için tanımlanmış test hesabına tek tıkla geçiş yapabilir ve rol kısıtlamalarını inceleyebilirsiniz:
            </p>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              {DEMO_PRESETS.map(preset => {
                const isSelected = email === preset.email;
                return (
                  <button
                    key={preset.role}
                    type="button"
                    onClick={() => handleSelectPreset(preset.email)}
                    className={`p-3 rounded-xl border text-left transition-all ${
                      isSelected
                        ? 'border-brand-500 bg-brand-50/70 dark:bg-brand-950/40 ring-1 ring-brand-500'
                        : 'border-slate-200/80 dark:border-slate-800 bg-slate-50/60 dark:bg-slate-950/60 hover:bg-slate-100 dark:hover:bg-slate-800/80'
                    }`}
                  >
                    <div className="flex items-center justify-between mb-1">
                      <div className="flex items-center space-x-1.5 font-bold text-xs text-slate-900 dark:text-white">
                        {preset.icon}
                        <span>{preset.label}</span>
                      </div>
                      {isSelected && (
                        <CheckCircle2 className="w-3.5 h-3.5 text-brand-600 dark:text-brand-400" />
                      )}
                    </div>
                    <div className="text-[11px] font-mono text-slate-500 dark:text-slate-400 truncate">
                      {preset.email}
                    </div>
                  </button>
                );
              })}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
