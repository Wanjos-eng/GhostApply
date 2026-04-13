import { useState } from 'react';
import { Sidebar } from "./components/Sidebar";
import { TopAppBar } from "./components/TopAppBar";
import { DashboardView } from "./components/DashboardView";
import { BaseProfile } from "./components/BaseProfile";
import { DossierReports } from "./components/DossierReports";
import { HistoryView } from "./components/HistoryView";
import { Settings } from "./components/Settings";
import { ProspectedJobsView } from "./components/ProspectedJobsView";

function App() {
  const [activeScreen, setActiveScreen] = useState('dashboard');
  const isActive = (screen: string) => activeScreen === screen;

  return (
    <div className="flex h-screen w-full overflow-hidden bg-[#f9f9f9] text-[#1a1c1c] font-sans">
      <Sidebar activeScreen={activeScreen} setActiveScreen={setActiveScreen} />
      <main className="flex-1 flex flex-col min-w-0 h-screen">
        <TopAppBar />
        <div className={isActive('dashboard') ? 'flex-1 min-h-0' : 'hidden'}>
          <DashboardView />
        </div>
        <div className={isActive('history') ? 'flex-1 min-h-0' : 'hidden'}>
          <HistoryView />
        </div>
        <div className={isActive('prospected') ? 'flex-1 min-h-0' : 'hidden'}>
          <ProspectedJobsView />
        </div>
        <div className={isActive('profile') ? 'flex-1 min-h-0' : 'hidden'}>
          <BaseProfile />
        </div>
        <div className={isActive('reports') ? 'flex-1 min-h-0' : 'hidden'}>
          <DossierReports />
        </div>
        <div className={isActive('settings') ? 'flex-1 min-h-0' : 'hidden'}>
          <Settings />
        </div>
      </main>
    </div>
  );
}

export default App;
