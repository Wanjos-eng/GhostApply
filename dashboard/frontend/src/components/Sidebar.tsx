

import { useEffect, useState } from 'react';

interface SidebarProps {
  activeScreen: string;
  setActiveScreen: (screen: string) => void;
}

interface SystemStatus {
  database: string;
  database_detail: string;
  database_path: string;
  cohere: string;
  groq: string;
  gemini: string;
  imap: string;
}

const defaultSystemStatus: SystemStatus = {
  database: "...",
  database_detail: "Aguardando diagnóstico...",
  database_path: "",
  cohere: "...",
  groq: "...",
  gemini: "...",
  imap: "..."
};

function normalizeSystemStatus(raw: unknown): SystemStatus {
  if (!raw || typeof raw !== 'object') {
    return defaultSystemStatus;
  }

  const obj = raw as Record<string, unknown>;
  const pick = (key: keyof SystemStatus) => {
    const value = obj[key];
    return typeof value === 'string' && value.trim() !== '' ? value : defaultSystemStatus[key];
  };

  return {
    database: pick('database'),
    database_detail: pick('database_detail'),
    database_path: pick('database_path'),
    cohere: pick('cohere'),
    groq: pick('groq'),
    gemini: pick('gemini'),
    imap: pick('imap')
  };
}

export function Sidebar({ activeScreen, setActiveScreen }: SidebarProps) {
  const [systemStatus, setSystemStatus] = useState<SystemStatus>(defaultSystemStatus);

  useEffect(() => {
    let inFlight = false;

    const fetchSystemStatus = async () => {
      if (inFlight) {
        return;
      }
      if ((window as any).go?.main?.App?.GetSystemStatus) {
        try {
          inFlight = true;
          const status = await (window as any).go.main.App.GetSystemStatus();
          setSystemStatus(normalizeSystemStatus(status));
        } catch (err) {
          console.error("Error loading system status:", err);
          setSystemStatus(defaultSystemStatus);
        } finally {
          inFlight = false;
        }
      }
    };

    fetchSystemStatus();
    const interval = setInterval(fetchSystemStatus, 30000); // Atualiza a cada 30s.
    const onSettingsSaved = () => {
      fetchSystemStatus();
    };
    window.addEventListener('ghostapply:settings-saved', onSettingsSaved);

    return () => {
      clearInterval(interval);
      window.removeEventListener('ghostapply:settings-saved', onSettingsSaved);
    };
  }, []);

  const getNavClass = (screen: string) => {
    const baseClass = "flex items-center gap-3 py-2 px-2 font-headline text-[0.875rem] leading-none transition-colors cursor-pointer ";
    if (activeScreen === screen) {
      return baseClass + "text-zinc-900 font-bold border-r-2 border-blue-600";
    }
    return baseClass + "text-zinc-500 hover:bg-zinc-50 font-normal";
  };

  return (
    <aside className="w-[240px] h-screen bg-white border-r border-zinc-100 flex flex-col py-6 px-4 shrink-0">
      <nav className="space-y-1">
        <button onClick={() => setActiveScreen('dashboard')} className={`w-full text-left ${getNavClass('dashboard')}`}>
          <span className="material-symbols-outlined text-[1.25rem]">dashboard</span>
          Dashboard
        </button>
        <button onClick={() => setActiveScreen('history')} className={`w-full text-left ${getNavClass('history')}`}>
          <span className="material-symbols-outlined text-[1.25rem]">history</span>
          History
        </button>
        <button onClick={() => setActiveScreen('prospected')} className={`w-full text-left ${getNavClass('prospected')}`}>
          <span className="material-symbols-outlined text-[1.25rem]">travel_explore</span>
          Prospected Jobs
        </button>
        <button onClick={() => setActiveScreen('profile')} className={`w-full text-left ${getNavClass('profile')}`}>
          <span className="material-symbols-outlined text-[1.25rem]">account_circle</span>
          Base Profile
        </button>
        <button onClick={() => setActiveScreen('reports')} className={`w-full text-left ${getNavClass('reports')}`}>
          <span className="material-symbols-outlined text-[1.25rem]">description</span>
          Dossier Reports
        </button>
        <button onClick={() => setActiveScreen('settings')} className={`w-full text-left ${getNavClass('settings')}`}>
          <span className="material-symbols-outlined text-[1.25rem]">settings</span>
          Settings
        </button>
      </nav>
      <div className="mt-auto px-2">
        <div className="p-3 bg-surface-container-low rounded-lg">
          <p className="text-[0.6875rem] font-mono text-zinc-400 uppercase mb-2">System Status</p>
          <div className="space-y-1">
            <div className="flex justify-between text-[0.65rem] font-mono">
              <span className="text-zinc-500">DB</span>
              <span className="text-blue-600 font-semibold">{systemStatus.database}</span>
            </div>
            <div className="text-[0.58rem] leading-tight text-zinc-500 font-mono mt-1 break-words">
              {systemStatus.database_detail}
            </div>
            {systemStatus.database_path && (
              <div className="text-[0.55rem] leading-tight text-zinc-400 font-mono break-all">
                {systemStatus.database_path}
              </div>
            )}
            <div className="flex justify-between text-[0.65rem] font-mono">
              <span className="text-zinc-500">Cohere</span>
              <span className="text-blue-600 font-semibold">{systemStatus.cohere}</span>
            </div>
            <div className="flex justify-between text-[0.65rem] font-mono">
              <span className="text-zinc-500">Groq</span>
              <span className="text-blue-600 font-semibold">{systemStatus.groq}</span>
            </div>
            <div className="flex justify-between text-[0.65rem] font-mono">
              <span className="text-zinc-500">Gemini</span>
              <span className="text-blue-600 font-semibold">{systemStatus.gemini}</span>
            </div>
            <div className="flex justify-between text-[0.65rem] font-mono">
              <span className="text-zinc-500">IMAP</span>
              <span className="text-blue-600 font-semibold">{systemStatus.imap}</span>
            </div>
          </div>
        </div>
      </div>
    </aside>
  );
}
