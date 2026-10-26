import React, { useState, useMemo } from 'react';
import {
  DollarSign,
  Users,
  BarChart3,
  Activity,
  Search,
  Filter,
  Building2,
  ShieldCheck,
} from 'lucide-react';
import { InvoiceItem, ScreenState } from '../../types';
import { StatCard } from '../common/StatCard';
import { EmptyState } from '../common/EmptyState';
import { ErrorBanner } from '../common/ErrorBanner';
import { SkeletonStat, SkeletonTable } from '../common/LoadingSkeleton';

interface ManagerViewProps {
  screenState: ScreenState;
  onSimulateStateChange: (state: ScreenState) => void;
  isAccountantView?: boolean;
}

const INITIAL_INVOICES: InvoiceItem[] = [
  {
    number: 'FIT-2026-IST01-000001',
    member: 'Bilal Erkan Taşkın',
    amount: '₺1.500,00',
    status: 'PAID',
    date: '25.10.2026',
    branch: 'Beşiktaş Merkez',
  },
  {
    number: 'FIT-2026-IST01-000002',
    member: 'Ahmet Yılmaz',
    amount: '₺2.250,00',
    status: 'PAID',
    date: '25.10.2026',
    branch: 'Beşiktaş Merkez',
  },
  {
    number: 'FIT-2026-IST01-000003',
    member: 'Zeynep Kaya',
    amount: '₺1.500,00',
    status: 'PAID',
    date: '25.10.2026',
    branch: 'Kadıköy Rıhtım',
  },
  {
    number: 'FIT-2026-IST01-000004',
    member: 'Emre Çelik',
    amount: '₺750,00',
    status: 'PAID',
    date: '25.10.2026',
    branch: 'Maslak Tech',
  },
  {
    number: 'FIT-2026-IST01-000005',
    member: 'Ayşe Öztürk',
    amount: '₺3.800,00',
    status: 'PAID',
    date: '25.10.2026',
    branch: 'Çankaya Ankara',
  },
  {
    number: 'FIT-2026-IST01-000006',
    member: 'Canan Güneş',
    amount: '₺1.500,00',
    status: 'PAID',
    date: '26.10.2026',
    branch: 'Alsancak İzmir',
  },
];

const BRANCH_STATS = [
  { name: 'Beşiktaş Merkez', city: 'İstanbul', activeMembers: 5240, occupancy: '%88', revenue: '₺234.000' },
  { name: 'Kadıköy Rıhtım', city: 'İstanbul', activeMembers: 4890, occupancy: '%82', revenue: '₺198.500' },
  { name: 'Maslak Tech', city: 'İstanbul', activeMembers: 3950, occupancy: '%74', revenue: '₺175.000' },
  { name: 'Çankaya', city: 'Ankara', activeMembers: 3210, occupancy: '%69', revenue: '₺126.200' },
  { name: 'Alsancak', city: 'İzmir', activeMembers: 2710, occupancy: '%65', revenue: '₺108.800' },
];

export const ManagerView: React.FC<ManagerViewProps> = ({
  screenState,
  onSimulateStateChange,
  isAccountantView = false,
}) => {
  const [invoices] = useState<InvoiceItem[]>(INITIAL_INVOICES);
  const [searchQuery, setSearchQuery] = useState('');
  const [branchFilter, setBranchFilter] = useState('ALL');

  const filteredInvoices = useMemo(() => {
    return invoices.filter(inv => {
      const matchesSearch =
        inv.number.toLowerCase().includes(searchQuery.toLowerCase()) ||
        inv.member.toLowerCase().includes(searchQuery.toLowerCase());
      const matchesBranch =
        branchFilter === 'ALL' || inv.branch === branchFilter;
      return matchesSearch && matchesBranch;
    });
  }, [invoices, searchQuery, branchFilter]);

  const branches = ['ALL', 'Beşiktaş Merkez', 'Kadıköy Rıhtım', 'Maslak Tech', 'Çankaya Ankara', 'Alsancak İzmir'];

  return (
    <div className="space-y-6">
      {screenState === 'error' && (
        <ErrorBanner
          title="Finansal Raporlar Yüklenemedi"
          message="PostgreSQL pgxpool havuzundan analitik veri çekerken zaman aşımı oluştu. Lütfen tekrar deneyin."
          onRetry={() => onSimulateStateChange('ready')}
          onDismiss={() => onSimulateStateChange('ready')}
        />
      )}

      {/* Top Stat Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {screenState === 'loading' ? (
          <>
            <SkeletonStat />
            <SkeletonStat />
            <SkeletonStat />
            <SkeletonStat />
          </>
        ) : (
          <>
            <StatCard
              title="Aylık Toplam Ciro"
              value="₺842.500,00"
              subtitle="Tüm tutarlar BIGINT kuruş saklanır"
              accentColor="emerald"
              icon={<DollarSign className="w-5 h-5" />}
              trend={{ value: '↑ %14,2 Artış', isPositive: true }}
            />
            <StatCard
              title="Kayıtlı Aktif Üyeler"
              value="20.000"
              subtitle="Deterministik Tohum (Seed=42)"
              accentColor="indigo"
              icon={<Users className="w-5 h-5" />}
              trend={{ value: '5 Şube Entegre', isPositive: true }}
            />
            <StatCard
              title="Pik Saat Doluluğu"
              value="19:00 - 20:00"
              subtitle="Günde ~420 anlık geçiş"
              accentColor="violet"
              icon={<BarChart3 className="w-5 h-5" />}
              trend={{ value: '%88 Maksimum', isPositive: true }}
            />
            <StatCard
              title="Turnike Gecikmesi (p95)"
              value="4.2 ms"
              subtitle="Hedef: < 50ms (%100 Başarı)"
              accentColor="blue"
              icon={<Activity className="w-5 h-5" />}
              trend={{ value: 'p95 < 5ms', isPositive: true }}
            />
          </>
        )}
      </div>

      {/* Gapless Invoices Section (Görev 4 Kanıtı) */}
      <div className="bg-white dark:bg-slate-900 border border-slate-200/80 dark:border-slate-800 rounded-2xl p-5 sm:p-6 shadow-sm hover:shadow-md transition-all space-y-5">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-slate-100 dark:border-slate-800 pb-4">
          <div>
            <div className="flex items-center space-x-2">
              <h2 className="font-bold text-sm sm:text-base text-slate-900 dark:text-white">
                {isAccountantView
                  ? 'Muhasebe & Boşluksuz Sıralı Fatura Listesi'
                  : 'Boşluksuz Sıralı Fatura Listesi (Gapless Invoices)'}
              </h2>
              <span className="text-[10px] px-2 py-0.5 rounded-full bg-brand-50 text-brand-700 dark:bg-brand-950/60 dark:text-brand-300 border border-brand-200 dark:border-brand-800 font-bold font-mono">
                T-4 Modülü
              </span>
            </div>
            <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
              <code>SELECT ... FOR UPDATE</code> ile korunan <code>invoice_sequences</code> tablosundan üretilen çift ve boşluksuz seri
            </p>
          </div>
          <div className="inline-flex items-center space-x-1.5 px-3 py-1.5 rounded-full bg-emerald-50 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800 text-xs font-bold">
            <ShieldCheck className="w-4 h-4" />
            <span>Sıfır Boşluk (Zero Gap Guaranteed)</span>
          </div>
        </div>

        {/* Filter & Search Bar */}
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
          <div className="relative flex-1">
            <Search className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <input
              type="text"
              value={searchQuery}
              onChange={e => setSearchQuery(e.target.value)}
              placeholder="Fatura no (FIT-2026-...) veya üye adı ara..."
              className="w-full pl-9 pr-3 py-2 rounded-xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 text-xs sm:text-sm text-slate-900 dark:text-slate-100 placeholder-slate-400 focus:outline-none focus:border-brand-500 transition"
            />
          </div>
          <div className="flex items-center space-x-2 overflow-x-auto pb-1 sm:pb-0">
            <span className="text-xs text-slate-400 flex items-center space-x-1 hidden sm:inline-flex">
              <Filter className="w-3.5 h-3.5" />
              <span>Şube:</span>
            </span>
            <select
              value={branchFilter}
              onChange={e => setBranchFilter(e.target.value)}
              className="bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 rounded-xl px-3 py-2 text-xs font-medium text-slate-700 dark:text-slate-200 focus:outline-none focus:border-brand-500"
            >
              {branches.map(b => (
                <option key={b} value={b}>
                  {b === 'ALL' ? 'Tüm Şubeler (5 Şube)' : b}
                </option>
              ))}
            </select>
          </div>
        </div>

        {/* Invoices Table / States */}
        {screenState === 'loading' ? (
          <SkeletonTable rows={5} />
        ) : screenState === 'empty' || filteredInvoices.length === 0 ? (
          <EmptyState
            title="Fatura Kaydı Bulunamadı"
            description="Arama kriterlerinize uyan fatura bulunamadı. Filtreleri temizleyebilirsiniz."
            actionLabel="Filtreleri Sıfırla"
            onAction={() => {
              setSearchQuery('');
              setBranchFilter('ALL');
              onSimulateStateChange('ready');
            }}
          />
        ) : (
          <div className="overflow-x-auto rounded-xl border border-slate-200/80 dark:border-slate-800">
            <table className="w-full text-left text-xs text-slate-600 dark:text-slate-300">
              <thead className="text-[11px] uppercase bg-slate-50 dark:bg-slate-950 text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800 font-semibold">
                <tr>
                  <th className="py-3 px-3.5">Fatura No (Ardışık)</th>
                  <th className="py-3 px-3.5">Üye</th>
                  <th className="py-3 px-3.5">Şube</th>
                  <th className="py-3 px-3.5">Tutar (Kuruş/BIGINT)</th>
                  <th className="py-3 px-3.5">Durum</th>
                  <th className="py-3 px-3.5 text-right">Tarih</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 dark:divide-slate-800 font-mono">
                {filteredInvoices.map(inv => (
                  <tr
                    key={inv.number}
                    className="hover:bg-slate-50/80 dark:hover:bg-slate-800/50 transition"
                  >
                    <td className="py-3 px-3.5 font-bold text-brand-600 dark:text-brand-400">
                      {inv.number}
                    </td>
                    <td className="py-3 px-3.5 font-medium text-slate-900 dark:text-slate-100 font-sans">
                      {inv.member}
                    </td>
                    <td className="py-3 px-3.5 text-slate-500 dark:text-slate-400 font-sans">
                      {inv.branch}
                    </td>
                    <td className="py-3 px-3.5 font-bold text-emerald-600 dark:text-emerald-400">
                      {inv.amount}
                    </td>
                    <td className="py-3 px-3.5">
                      <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-emerald-50 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800">
                        {inv.status}
                      </span>
                    </td>
                    <td className="py-3 px-3.5 text-right text-slate-500 dark:text-slate-400">
                      {inv.date}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* 5 Branches Summary Grid */}
      <div className="bg-white dark:bg-slate-900 border border-slate-200/80 dark:border-slate-800 rounded-2xl p-5 sm:p-6 shadow-sm hover:shadow-md transition-all space-y-4">
        <div className="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-3">
          <div className="flex items-center space-x-2">
            <Building2 className="w-5 h-5 text-brand-500" />
            <h3 className="font-bold text-sm sm:text-base text-slate-900 dark:text-white">
              5 Şube Doluluk ve Performans Analitiği
            </h3>
          </div>
          <span className="text-xs text-slate-400 font-mono">Merkezi Veri Havuzu</span>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3.5 pt-1">
          {BRANCH_STATS.map((br, idx) => (
            <div
              key={idx}
              className="p-4 rounded-xl bg-slate-50 dark:bg-slate-950/60 border border-slate-200/80 dark:border-slate-800 space-y-2 hover:border-brand-300 dark:hover:border-slate-700 transition"
            >
              <div className="flex items-center justify-between">
                <span className="font-bold text-xs text-slate-900 dark:text-slate-100">
                  {br.name}
                </span>
                <span className="text-[10px] text-slate-400">{br.city}</span>
              </div>
              <div className="text-base font-extrabold text-brand-600 dark:text-brand-400">
                {br.revenue}
              </div>
              <div className="flex justify-between text-[11px] text-slate-500 dark:text-slate-400 pt-1 border-t border-slate-200/60 dark:border-slate-800">
                <span>Üye: {br.activeMembers.toLocaleString()}</span>
                <span className="font-semibold text-emerald-600 dark:text-emerald-400">
                  {br.occupancy}
                </span>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};
