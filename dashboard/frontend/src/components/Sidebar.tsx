import { useEffect, useState } from 'react';
import { LoadSettings } from "../../wailsjs/go/main/App";

interface SidebarProps {
  activeScreen: string;
  setActiveScreen: (screen: string) => void;
}

export function Sidebar({ activeScreen, setActiveScreen }: SidebarProps) {
  const [identity, setIdentity] = useState({ name: "Ghost Agent", initials: "GA" });

  useEffect(() => {
    const fetchIdentity = async () => {
      if ((window as any).go) {
        try {
          const cfg = await LoadSettings();
          if (cfg && cfg.imap_user) {
            const raw = cfg.imap_user.split('@')[0];
            const parts = raw.split(/[._-]/); // Split by common email separators
            const formattedName = parts.map(p => p.charAt(0).toUpperCase() + p.slice(1)).join(' ');
            
            const initials = parts.slice(0, 2).map(p => p.charAt(0).toUpperCase()).join('');
            setIdentity({ name: formattedName, initials: initials || raw.charAt(0).toUpperCase() });
          }
        } catch (err) {
          console.error("Error loading identity from settings:", err);
        }
      }
    };
    fetchIdentity();
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
      <div className="mb-8 px-2 flex items-center gap-3">
        <div className="w-9 h-9 rounded-full bg-blue-600 flex items-center justify-center text-white font-bold font-headline text-sm shadow-sm ring-2 ring-blue-100">
           {identity.initials}
        </div>
        <div className="flex-1 w-0">
          <p className="font-headline font-semibold text-sm text-zinc-900 leading-none truncate">{identity.name}</p>
          <div className="flex items-center gap-1.5 mt-1">
            <span className="w-1.5 h-1.5 rounded-full bg-green-500 shadow-[0_0_4px_rgba(34,197,94,0.6)]"></span>
            <p className="text-[0.6875rem] font-medium text-zinc-500 uppercase tracking-wider">Online</p>
          </div>
        </div>
      </div>
      <nav className="space-y-1">
        <button onClick={() => setActiveScreen('dashboard')} className={`w-full text-left ${getNavClass('dashboard')}`}>
          <span className="material-symbols-outlined text-[1.25rem]">dashboard</span>
          Dashboard
        </button>
        <button
          onClick={() => setActiveScreen('history')}
          className={`w-full flex items-center px-4 py-3 mb-2 rounded-xl border transition-all duration-300 shadow-sm
            ${activeScreen === 'history' 
              ? 'bg-[#1a1c1c] text-white border-transparent shadow-md translate-x-1' 
              : 'bg-white text-gray-500 border-gray-100 hover:border-gray-300 hover:text-[#1a1c1c]'
            }`}
        >
          <svg className="w-5 h-5 mr-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <span className="font-semibold text-sm tracking-wide">Histórico</span>
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
          <p className="text-[0.75rem] font-mono text-blue-600 font-bold">STABLE_V.2.4.0</p>
        </div>
      </div>
    </aside>
  );
}
