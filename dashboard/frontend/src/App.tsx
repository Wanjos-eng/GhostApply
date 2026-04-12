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

  return (
    <div className="flex min-h-screen w-full bg-[#f9f9f9] text-[#1a1c1c] font-sans">
      <Sidebar activeScreen={activeScreen} setActiveScreen={setActiveScreen} />
      <main className="flex-1 flex flex-col min-w-0 h-screen">
        <TopAppBar />
        {activeScreen === 'dashboard' && <DashboardView />}
        {activeScreen === 'history' && <HistoryView />}
        {activeScreen === 'prospected' && <ProspectedJobsView />}
        {activeScreen === 'profile' && <BaseProfile />}
        {activeScreen === 'reports' && <DossierReports />}
        {activeScreen === 'settings' && <Settings />}
      </main>
    </div>
  );
}

export default App;
