import React from 'react';
import { Loader2 } from 'lucide-react';

export const LoadingSpinner: React.FC<{ message?: string }> = ({
  message = 'Veriler güncelleniyor...',
}) => (
  <div className="flex flex-col items-center justify-center p-12 text-center space-y-3">
    <div className="relative">
      <div className="w-10 h-10 rounded-full border-2 border-brand-100 dark:border-brand-900 border-t-brand-600 animate-spin" />
      <Loader2 className="w-5 h-5 text-brand-600 absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 animate-pulse" />
    </div>
    <p className="text-xs sm:text-sm font-medium text-slate-500 dark:text-slate-400">
      {message}
    </p>
  </div>
);

export const SkeletonStat: React.FC = () => (
  <div className="p-5 rounded-2xl bg-white dark:bg-slate-900 border border-slate-200/80 dark:border-slate-800 shadow-sm animate-pulse space-y-3">
    <div className="flex items-center justify-between">
      <div className="h-3 w-24 bg-slate-200 dark:bg-slate-800 rounded" />
      <div className="w-8 h-8 rounded-xl bg-slate-200 dark:bg-slate-800" />
    </div>
    <div className="h-7 w-32 bg-slate-200 dark:bg-slate-800 rounded" />
    <div className="h-3 w-40 bg-slate-100 dark:bg-slate-800/60 rounded" />
  </div>
);

export const SkeletonCard: React.FC<{ lines?: number }> = ({ lines = 3 }) => (
  <div className="p-5 rounded-2xl bg-white dark:bg-slate-900 border border-slate-200/80 dark:border-slate-800 shadow-sm animate-pulse space-y-3">
    <div className="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-3">
      <div className="h-4 w-32 bg-slate-200 dark:bg-slate-800 rounded" />
      <div className="h-4 w-16 bg-slate-200 dark:bg-slate-800 rounded-full" />
    </div>
    <div className="space-y-2 pt-1">
      {Array.from({ length: lines }).map((_, i) => (
        <div
          key={i}
          className="h-3 bg-slate-100 dark:bg-slate-800/60 rounded"
          style={{ width: `${85 - i * 15}%` }}
        />
      ))}
    </div>
  </div>
);

export const SkeletonTable: React.FC<{ rows?: number }> = ({ rows = 4 }) => (
  <div className="p-5 rounded-2xl bg-white dark:bg-slate-900 border border-slate-200/80 dark:border-slate-800 shadow-sm animate-pulse space-y-4">
    <div className="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-4">
      <div className="h-4 w-40 bg-slate-200 dark:bg-slate-800 rounded" />
      <div className="h-8 w-24 bg-slate-100 dark:bg-slate-800 rounded-lg" />
    </div>
    <div className="space-y-3">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex items-center space-x-4 py-2 border-b border-slate-50 dark:border-slate-800/40">
          <div className="h-3.5 w-16 bg-slate-200 dark:bg-slate-800 rounded" />
          <div className="h-3.5 flex-1 bg-slate-100 dark:bg-slate-800/60 rounded" />
          <div className="h-3.5 w-20 bg-slate-200 dark:bg-slate-800 rounded" />
          <div className="h-5 w-16 bg-slate-100 dark:bg-slate-800 rounded-full" />
        </div>
      ))}
    </div>
  </div>
);
