import React, { useState } from 'react';
import {
  CheckCircle2,
  XCircle,
  Clock,
  Check,
} from 'lucide-react';
import { PTSession, ScreenState } from '../../types';
import { EmptyState } from '../common/EmptyState';
import { ErrorBanner } from '../common/ErrorBanner';
import { SkeletonCard } from '../common/LoadingSkeleton';

interface TrainerViewProps {
  screenState: ScreenState;
  onSimulateStateChange: (state: ScreenState) => void;
}

interface StudentAttendance {
  id: string;
  name: string;
  card: string;
  status: 'ATTENDED' | 'NO_SHOW' | 'PENDING';
}

const INITIAL_STUDENTS: StudentAttendance[] = [
  { id: '1', name: 'Bilal Erkan Taşkın', card: 'CARD-DEMO-001', status: 'ATTENDED' },
  { id: '2', name: 'Oğuzhan Çetin', card: 'member_00042', status: 'PENDING' },
  { id: '3', name: 'Gizem Yurt', card: 'member_00088', status: 'PENDING' },
  { id: '4', name: 'Can Demir', card: 'member_00105', status: 'NO_SHOW' },
];

const INITIAL_PT_SESSIONS: PTSession[] = [
  {
    id: 'pt1',
    client: 'Bilal Erkan Taşkın',
    package: '10 Seans Özel PT Paketi',
    slot: 'Bugün 17:00 - 18:00',
    status: 'CONFIRMED',
    remaining: '8 / 10 Seans',
  },
  {
    id: 'pt2',
    client: 'Seda Korkmaz',
    package: '20 Seans İleri Seviye PT',
    slot: 'Bugün 18:30 - 19:30',
    status: 'CONFIRMED',
    remaining: '14 / 20 Seans',
  },
  {
    id: 'pt3',
    client: 'Mehmet Yılmaz',
    package: '5 Seans Kondisyon PT',
    slot: 'Yarın 11:00 - 12:00',
    status: 'CONFIRMED',
    remaining: '2 / 5 Seans',
  },
];

export const TrainerView: React.FC<TrainerViewProps> = ({
  screenState,
  onSimulateStateChange,
}) => {
  const [students, setStudents] = useState<StudentAttendance[]>(INITIAL_STUDENTS);
  const [ptSessions, setPtSessions] = useState<PTSession[]>(INITIAL_PT_SESSIONS);
  const [toastMessage, setToastMessage] = useState<string | null>(null);

  const markAttendance = (id: string, newStatus: 'ATTENDED' | 'NO_SHOW') => {
    setStudents(prev =>
      prev.map(s => (s.id === id ? { ...s, status: newStatus } : s))
    );
    const target = students.find(s => s.id === id);
    setToastMessage(
      `${target?.name || 'Üye'} için yoklama durumu "${
        newStatus === 'ATTENDED' ? 'Geldi (Katıldı)' : 'Gelmeyen (No-Show)'
      }" olarak kaydedildi.`
    );
    setTimeout(() => setToastMessage(null), 3500);
  };

  const completePTSession = (id: string) => {
    setPtSessions(prev =>
      prev.map(s => (s.id === id ? { ...s, status: 'COMPLETED' } : s))
    );
    setToastMessage('PT seansı tamamlandı olarak işaretlendi ve seans bakiyesinden 1 hak düşüldü.');
    setTimeout(() => setToastMessage(null), 3500);
  };

  const attendedCount = students.filter(s => s.status === 'ATTENDED').length;
  const noShowCount = students.filter(s => s.status === 'NO_SHOW').length;

  return (
    <div className="space-y-6">
      {/* Toast Notice */}
      {toastMessage && (
        <div className="p-4 rounded-xl bg-emerald-50 dark:bg-emerald-950/60 border border-emerald-200 dark:border-emerald-800 text-emerald-800 dark:text-emerald-200 text-xs sm:text-sm flex items-center space-x-2.5 shadow-sm animate-in fade-in slide-in-from-top-2 duration-200">
          <CheckCircle2 className="w-5 h-5 text-emerald-600 dark:text-emerald-400 flex-shrink-0" />
          <span className="font-medium">{toastMessage}</span>
        </div>
      )}

      {screenState === 'error' && (
        <ErrorBanner
          title="Eğitmen Programı Alınamadı"
          message="Randevu servisiyle bağlantı kurulamadı. Lütfen tekrar deneyin."
          onRetry={() => onSimulateStateChange('ready')}
          onDismiss={() => onSimulateStateChange('ready')}
        />
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Left: Group Class Attendance */}
        <div className="bg-white dark:bg-slate-900 border border-slate-200/80 dark:border-slate-800 rounded-2xl p-5 sm:p-6 shadow-sm hover:shadow-md transition-all space-y-5">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-slate-100 dark:border-slate-800 pb-4">
            <div>
              <div className="flex items-center space-x-2">
                <h2 className="font-bold text-sm sm:text-base text-slate-900 dark:text-white">
                  Grup Dersi Yoklaması
                </h2>
                <span className="text-[10px] px-2 py-0.5 rounded-full bg-brand-50 text-brand-700 dark:bg-brand-950/60 dark:text-brand-300 border border-brand-200 dark:border-brand-800 font-semibold">
                  F-4 Modülü
                </span>
              </div>
              <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                Yoklama onaylandığında seans hakları otomatik düşülür
              </p>
            </div>
            <div className="text-xs font-semibold px-3 py-1.5 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-200 border border-slate-200 dark:border-slate-700">
              19:00 Spinning (Beşiktaş)
            </div>
          </div>

          {/* Quick Metrics */}
          <div className="grid grid-cols-3 gap-2 text-center text-xs">
            <div className="p-2.5 rounded-xl bg-slate-50 dark:bg-slate-950/60 border border-slate-100 dark:border-slate-800">
              <span className="text-slate-400 block text-[10px]">Kayıtlı</span>
              <span className="font-bold text-slate-900 dark:text-white text-sm">
                {students.length}
              </span>
            </div>
            <div className="p-2.5 rounded-xl bg-emerald-50/70 dark:bg-emerald-950/40 border border-emerald-100 dark:border-emerald-800/60">
              <span className="text-emerald-700 dark:text-emerald-400 block text-[10px]">
                Katıldı
              </span>
              <span className="font-bold text-emerald-700 dark:text-emerald-300 text-sm">
                {attendedCount}
              </span>
            </div>
            <div className="p-2.5 rounded-xl bg-rose-50/70 dark:bg-rose-950/40 border border-rose-100 dark:border-rose-800/60">
              <span className="text-rose-700 dark:text-rose-400 block text-[10px]">
                Gelmeyen
              </span>
              <span className="font-bold text-rose-700 dark:text-rose-300 text-sm">
                {noShowCount}
              </span>
            </div>
          </div>

          {screenState === 'loading' ? (
            <SkeletonCard lines={4} />
          ) : screenState === 'empty' || students.length === 0 ? (
            <EmptyState
              title="Yoklama Listesi Boş"
              description="Bu ders için henüz kayıtlı üye bulunmamaktadır."
              actionLabel="Listeyi Sıfırla"
              onAction={() => setStudents(INITIAL_STUDENTS)}
            />
          ) : (
            <div className="space-y-3">
              {students.map(student => (
                <div
                  key={student.id}
                  className="p-3.5 sm:p-4 rounded-xl bg-slate-50 dark:bg-slate-950/60 border border-slate-200/80 dark:border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-3 hover:border-brand-200 dark:hover:border-slate-700 transition"
                >
                  <div>
                    <div className="font-bold text-xs sm:text-sm text-slate-900 dark:text-slate-100">
                      {student.name}
                    </div>
                    <div className="text-[11px] text-slate-400 font-mono mt-0.5">
                      {student.card}
                    </div>
                  </div>

                  <div className="flex items-center space-x-2">
                    <button
                      onClick={() => markAttendance(student.id, 'ATTENDED')}
                      className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition flex items-center space-x-1 ${
                        student.status === 'ATTENDED'
                          ? 'bg-emerald-600 text-white shadow-xs'
                          : 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300 hover:bg-emerald-100'
                      }`}
                    >
                      <Check className="w-3.5 h-3.5" />
                      <span>Katıldı</span>
                    </button>
                    <button
                      onClick={() => markAttendance(student.id, 'NO_SHOW')}
                      className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition flex items-center space-x-1 ${
                        student.status === 'NO_SHOW'
                          ? 'bg-rose-600 text-white shadow-xs'
                          : 'bg-rose-50 text-rose-700 dark:bg-rose-950/50 dark:text-rose-300 hover:bg-rose-100'
                      }`}
                    >
                      <XCircle className="w-3.5 h-3.5" />
                      <span>No-Show</span>
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Right: Personal Training Sessions */}
        <div className="bg-white dark:bg-slate-900 border border-slate-200/80 dark:border-slate-800 rounded-2xl p-5 sm:p-6 shadow-sm hover:shadow-md transition-all space-y-5">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-slate-100 dark:border-slate-800 pb-4">
            <div>
              <div className="flex items-center space-x-2">
                <h2 className="font-bold text-sm sm:text-base text-slate-900 dark:text-white">
                  Kişisel Antrenman (PT) Randevuları
                </h2>
                <span className="text-[10px] px-2 py-0.5 rounded-full bg-violet-50 text-violet-700 dark:bg-violet-950/60 dark:text-violet-300 border border-violet-200 dark:border-violet-800 font-semibold">
                  F-5 Modülü
                </span>
              </div>
              <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                Birebir koçluk ve antrenman tamamlama seansları
              </p>
            </div>
            <div className="text-xs font-mono text-violet-600 dark:text-violet-400 bg-violet-50 dark:bg-violet-950/60 px-3 py-1.5 rounded-xl border border-violet-200 dark:border-violet-800">
              3 Aktif Seans
            </div>
          </div>

          {screenState === 'loading' ? (
            <SkeletonCard lines={4} />
          ) : screenState === 'empty' || ptSessions.length === 0 ? (
            <EmptyState
              title="Bugün PT Randevusu Yok"
              description="Planlanmış antrenman bulunmamaktadır."
              actionLabel="Listeyi Yenile"
              onAction={() => setPtSessions(INITIAL_PT_SESSIONS)}
            />
          ) : (
            <div className="space-y-3.5">
              {ptSessions.map(session => (
                <div
                  key={session.id}
                  className="p-4 rounded-xl bg-slate-50 dark:bg-slate-950/60 border border-slate-200/80 dark:border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-3 hover:border-violet-300 dark:hover:border-slate-700 transition"
                >
                  <div className="space-y-1">
                    <div className="flex items-center space-x-2">
                      <span className="font-bold text-xs sm:text-sm text-slate-900 dark:text-slate-100">
                        {session.client}
                      </span>
                      {session.status === 'COMPLETED' && (
                        <span className="text-[10px] px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800 font-bold">
                          TAMAMLANDI
                        </span>
                      )}
                    </div>
                    <div className="text-xs font-mono text-brand-600 dark:text-brand-400 flex items-center space-x-1.5">
                      <Clock className="w-3.5 h-3.5" />
                      <span>{session.slot}</span>
                    </div>
                    <div className="text-[11px] text-slate-500 dark:text-slate-400">
                      {session.package} •{' '}
                      <span className="font-semibold text-slate-700 dark:text-slate-300">
                        {session.remaining}
                      </span>
                    </div>
                  </div>

                  {session.status === 'CONFIRMED' ? (
                    <button
                      onClick={() => completePTSession(session.id)}
                      className="px-3.5 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 active:bg-violet-700 text-white text-xs font-semibold shadow-xs transition"
                    >
                      Tamamlandı Olarak İşaretle
                    </button>
                  ) : (
                    <span className="text-xs text-emerald-600 dark:text-emerald-400 font-semibold flex items-center space-x-1">
                      <CheckCircle2 className="w-4 h-4" />
                      <span>İşlendi</span>
                    </span>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
