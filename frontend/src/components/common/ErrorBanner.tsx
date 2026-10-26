import React from 'react';
import { AlertTriangle, RefreshCw, X } from 'lucide-react';

interface ErrorBannerProps {
  title?: string;
  message: string;
  onRetry?: () => void;
  onDismiss?: () => void;
  className?: string;
}

export const ErrorBanner: React.FC<ErrorBannerProps> = ({
  title = 'Veri Yükleme Hatası',
  message,
  onRetry,
  onDismiss,
  className = '',
}) => {
  return (
    <div
      role="alert"
      className={`relative p-4 sm:p-5 rounded-2xl bg-rose-50/90 dark:bg-rose-950/40 border border-rose-200 dark:border-rose-800/80 text-rose-900 dark:text-rose-100 shadow-sm transition-all ${className}`}
    >
      <div className="flex items-start space-x-3 sm:space-x-4">
        <div className="p-2 bg-rose-100 dark:bg-rose-900/60 rounded-xl text-rose-600 dark:text-rose-400 flex-shrink-0 mt-0.5">
          <AlertTriangle className="w-5 h-5" />
        </div>
        <div className="flex-1 pr-6">
          <h4 className="text-sm font-semibold text-rose-950 dark:text-rose-100 mb-1">
            {title}
          </h4>
          <p className="text-xs sm:text-sm text-rose-700 dark:text-rose-300 leading-relaxed">
            {message}
          </p>
          {onRetry && (
            <div className="mt-3">
              <button
                onClick={onRetry}
                className="inline-flex items-center space-x-1.5 px-3 py-1.5 rounded-lg bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold shadow-xs transition"
              >
                <RefreshCw className="w-3.5 h-3.5 animate-spin-reverse" />
                <span>Yeniden Dene</span>
              </button>
            </div>
          )}
        </div>
        {onDismiss && (
          <button
            onClick={onDismiss}
            aria-label="Kapat"
            className="absolute top-4 right-4 p-1.5 rounded-lg text-rose-400 hover:text-rose-600 dark:hover:text-rose-200 hover:bg-rose-100 dark:hover:bg-rose-900/40 transition"
          >
            <X className="w-4 h-4" />
          </button>
        )}
      </div>
    </div>
  );
};
