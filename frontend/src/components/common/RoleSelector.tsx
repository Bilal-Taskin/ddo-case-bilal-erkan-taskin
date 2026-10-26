import React from 'react';
import {
  Shield,
  Activity,
  Calendar,
  Building2,
  FileText,
  CreditCard,
  MapPin,
  ExternalLink,
  LogOut,
} from 'lucide-react';
import { AuthUser, RoleKey, UserProfile } from '../../types';

export const DEMO_PROFILES: Record<RoleKey, UserProfile> = {
  MEMBER: {
    email: 'member@fitgiris.com',
    role: 'Üye (Member)',
    name: 'Bilal Erkan Taşkın',
    branch: 'Beşiktaş Merkez',
    badge: 'Gold Üyelik',
  },
  RECEPTION: {
    email: 'reception@fitgiris.com',
    role: 'Resepsiyon (Reception)',
    name: 'Burak Demir',
    branch: 'Beşiktaş Turnike Terminali',
    badge: 'Danışma Görevlisi',
  },
  TRAINER: {
    email: 'trainer@fitgiris.com',
    role: 'Eğitmen (Trainer)',
    name: 'Mert Aksoy',
    branch: 'Beşiktaş & Kadıköy',
    badge: 'Baş Antrenör (PT & Grup)',
  },
  MANAGER: {
    email: 'manager@fitgiris.com',
    role: 'Salon Müdürü (Manager)',
    name: 'Caner Öz',
    branch: 'Tüm 5 Şube Yönetimi',
    badge: 'Genel Koordinatör',
  },
  ACCOUNTANT: {
    email: 'accountant@fitgiris.com',
    role: 'Muhasebe (Accountant)',
    name: 'Selin Kaya',
    branch: 'Finans & Fatura Merkezi',
    badge: 'Mali İşler Sorumlusu',
  },
};

interface UserPersonaBannerProps {
  user: AuthUser;
  onLogout: () => void;
}

export const UserPersonaBanner: React.FC<UserPersonaBannerProps> = ({
  user,
  onLogout,
}) => {
  const profile = DEMO_PROFILES[user.role] || {
    email: user.email,
    role: user.role,
    name: `${user.first_name} ${user.last_name}`,
    branch: 'Beşiktaş Merkez',
    badge: user.role,
  };

  const getRoleIcon = (role: RoleKey) => {
    switch (role) {
      case 'MEMBER':
        return <Shield className="w-5 h-5 text-emerald-500" />;
      case 'RECEPTION':
        return <Activity className="w-5 h-5 text-blue-500" />;
      case 'TRAINER':
        return <Calendar className="w-5 h-5 text-amber-500" />;
      case 'MANAGER':
        return <Building2 className="w-5 h-5 text-brand-500" />;
      case 'ACCOUNTANT':
        return <CreditCard className="w-5 h-5 text-violet-500" />;
    }
  };

  return (
    <div className="space-y-3">
      {/* Active User Persona Banner */}
      <div className="p-4 sm:p-5 rounded-2xl bg-white dark:bg-slate-900 border border-slate-200/80 dark:border-slate-800 shadow-sm flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div className="flex items-center space-x-3.5">
          <div className="w-12 h-12 rounded-2xl bg-gradient-to-br from-brand-500 to-violet-600 text-white font-extrabold text-base flex items-center justify-center shadow-sm flex-shrink-0">
            {profile.name.charAt(0)}
          </div>
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-base sm:text-lg font-bold text-slate-900 dark:text-white">
                {profile.name}
              </h1>
              <span className="text-[11px] px-2.5 py-0.5 rounded-full bg-brand-50 text-brand-700 dark:bg-brand-950/60 dark:text-brand-300 border border-brand-200 dark:border-brand-800 font-semibold flex items-center space-x-1">
                {getRoleIcon(user.role)}
                <span>{profile.role}</span>
              </span>
            </div>
            <div className="flex flex-wrap items-center gap-3 text-xs text-slate-500 dark:text-slate-400 font-mono mt-0.5">
              <span>{user.email}</span>
              <span className="hidden sm:inline">•</span>
              <span className="flex items-center space-x-1 text-slate-600 dark:text-slate-300">
                <MapPin className="w-3.5 h-3.5 text-brand-500" />
                <span>{profile.branch}</span>
              </span>
            </div>
          </div>
        </div>

        <div className="flex items-center space-x-2 text-xs">
          <a
            href="http://localhost:8080/docs"
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center space-x-1.5 px-3 py-2 rounded-xl bg-slate-50 dark:bg-slate-800 text-slate-700 dark:text-slate-200 border border-slate-200 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-700 transition shadow-xs"
          >
            <FileText className="w-3.5 h-3.5 text-brand-500" />
            <span>OpenAPI Dokümanı</span>
            <ExternalLink className="w-3 h-3 text-slate-400" />
          </a>

          <button
            onClick={onLogout}
            className="inline-flex items-center space-x-1.5 px-3 py-2 rounded-xl bg-rose-50 hover:bg-rose-100 dark:bg-rose-950/50 dark:hover:bg-rose-900/50 text-rose-700 dark:text-rose-300 border border-rose-200 dark:border-rose-800 transition font-semibold"
          >
            <LogOut className="w-3.5 h-3.5" />
            <span>Farklı Rol ile Gir</span>
          </button>
        </div>
      </div>
    </div>
  );
};
