import { useState, useEffect } from 'react';
import { AuthUser, AuthTokens, ScreenState } from './types';
import { Header } from './components/common/Header';
import { UserPersonaBanner } from './components/common/RoleSelector';
import { LoginView } from './components/views/LoginView';
import { MemberView } from './components/views/MemberView';
import { ReceptionView } from './components/views/ReceptionView';
import { TrainerView } from './components/views/TrainerView';
import { ManagerView } from './components/views/ManagerView';

export default function App() {
  // Theme State (Default: Modern Light SaaS)
  const [isDarkMode, setIsDarkMode] = useState<boolean>(() => {
    return localStorage.getItem('fitgiris_theme') === 'dark';
  });

  // Authenticated User & Tokens (Persisted in localStorage)
  const [authUser, setAuthUser] = useState<AuthUser | null>(() => {
    const saved = localStorage.getItem('fitgiris_user');
    if (saved) {
      try {
        return JSON.parse(saved);
      } catch {
        return null;
      }
    }
    return null;
  });

  // Screen State for Loading / Empty / Error demonstration
  const [screenState, setScreenState] = useState<ScreenState>('ready');

  // Shared Card Token for Turnstile Pass Simulation
  const [cardToken, setCardToken] = useState('CARD-DEMO-001');

  // Mobile Drawer State
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

  // Sync dark mode class with html element
  useEffect(() => {
    const root = document.documentElement;
    if (isDarkMode) {
      root.classList.add('dark');
      localStorage.setItem('fitgiris_theme', 'dark');
    } else {
      root.classList.remove('dark');
      localStorage.setItem('fitgiris_theme', 'light');
    }
  }, [isDarkMode]);

  const toggleDarkMode = () => setIsDarkMode(prev => !prev);

  const handleLoginSuccess = (user: AuthUser, tokens: AuthTokens) => {
    setAuthUser(user);
    localStorage.setItem('fitgiris_user', JSON.stringify(user));
    localStorage.setItem('fitgiris_token', tokens.access_token);
    setScreenState('ready');
  };

  const handleLogout = () => {
    setAuthUser(null);
    localStorage.removeItem('fitgiris_user');
    localStorage.removeItem('fitgiris_token');
    setScreenState('ready');
  };

  // If user is not authenticated, show strict Login View
  if (!authUser) {
    return (
      <div className="min-h-screen bg-slate-50/70 dark:bg-slate-950 text-slate-900 dark:text-slate-100 flex flex-col font-sans transition-colors duration-200">
        <Header
          user={null}
          onLogout={handleLogout}
          isDarkMode={isDarkMode}
          onToggleDarkMode={toggleDarkMode}
          screenState="ready"
          onStateChange={setScreenState}
          isMobileMenuOpen={isMobileMenuOpen}
          onToggleMobileMenu={() => setIsMobileMenuOpen(prev => !prev)}
        />
        <main className="flex-1">
          <LoginView onLoginSuccess={handleLoginSuccess} />
        </main>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-slate-50/70 dark:bg-slate-950 text-slate-900 dark:text-slate-100 flex flex-col font-sans transition-colors duration-200">
      {/* Global Top Navbar with User info & Logout */}
      <Header
        user={authUser}
        onLogout={handleLogout}
        isDarkMode={isDarkMode}
        onToggleDarkMode={toggleDarkMode}
        screenState={screenState}
        onStateChange={setScreenState}
        isMobileMenuOpen={isMobileMenuOpen}
        onToggleMobileMenu={() => setIsMobileMenuOpen(prev => !prev)}
      />

      {/* Main Workspace */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6 sm:py-8 space-y-6">
        {/* Active User Persona Banner (No role switcher tabs!) */}
        <UserPersonaBanner user={authUser} onLogout={handleLogout} />

        {/* STRICT ROLE GUARD: ONLY the logged in role view is rendered */}
        <div className="transition-opacity duration-200">
          {authUser.role === 'MEMBER' && (
            <MemberView
              screenState={screenState}
              onSimulateStateChange={setScreenState}
              onNavigateToTurnstile={() => {
                alert('Turnike geçiş testi için Resepsiyon hesabı ile oturum açınız.');
              }}
            />
          )}

          {authUser.role === 'RECEPTION' && (
            <ReceptionView
              screenState={screenState}
              onSimulateStateChange={setScreenState}
              cardToken={cardToken}
              setCardToken={setCardToken}
            />
          )}

          {authUser.role === 'TRAINER' && (
            <TrainerView
              screenState={screenState}
              onSimulateStateChange={setScreenState}
            />
          )}

          {authUser.role === 'MANAGER' && (
            <ManagerView
              screenState={screenState}
              onSimulateStateChange={setScreenState}
              isAccountantView={false}
            />
          )}

          {authUser.role === 'ACCOUNTANT' && (
            <ManagerView
              screenState={screenState}
              onSimulateStateChange={setScreenState}
              isAccountantView={true}
            />
          )}
        </div>
      </main>

      {/* Modern SaaS Footer */}
      <footer className="border-t border-slate-200 dark:border-slate-800 bg-white/50 dark:bg-slate-900/40 backdrop-blur-sm py-6 transition-colors">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-slate-500 dark:text-slate-400">
          <div className="flex items-center space-x-2">
            <span className="font-semibold text-slate-800 dark:text-slate-200">
              FitGiriş Enterprise
            </span>
            <span>•</span>
            <span>DDO Yazılım Ekibi Alım Projesi</span>
            <span>•</span>
            <span>Geliştirici: <strong>Bilal Erkan Taşkın</strong></span>
          </div>

          <div className="flex items-center space-x-3 font-mono text-[11px]">
            <span className="px-2 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300">
              Go 1.22
            </span>
            <span className="px-2 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300">
              PostgreSQL 16
            </span>
            <span className="px-2 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300">
              React + Tailwind
            </span>
          </div>
        </div>
      </footer>
    </div>
  );
}
