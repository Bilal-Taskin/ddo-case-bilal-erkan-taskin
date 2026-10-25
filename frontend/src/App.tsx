import React from 'react';

export default function App() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-slate-900 text-white p-4">
      <div className="max-w-md w-full p-8 bg-slate-800 rounded-2xl shadow-xl text-center space-y-4 border border-slate-700">
        <div className="inline-flex p-3 bg-indigo-500/10 text-indigo-400 rounded-full">
          <svg className="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
        </div>
        <h1 className="text-2xl font-bold tracking-tight">FitGiriş</h1>
        <p className="text-slate-400 text-sm">
          Spor Salonu Üyelik, Giriş Kontrol ve Ders Yönetim Sistemi
        </p>
        <div className="pt-4 border-t border-slate-700">
          <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
            Faz 1 Altyapı Hazır
          </span>
        </div>
      </div>
    </div>
  );
}
