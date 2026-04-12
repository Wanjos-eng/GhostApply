import { useState } from 'react';
import { StartDaemon, UploadAndParseCV } from "../../wailsjs/go/main/App";

export function BaseProfile() {
  const [profile, setProfile] = useState({
    target_roles: [] as string[],
    core_stack: [] as string[],
    strictly_remote: true,
    min_salary_floor: "",
    apps_per_day: 0
  });

  const [status, setStatus] = useState("Daemon Ready");
  const [isParsing, setIsParsing] = useState(false);
  const [cvName, setCvName] = useState("");

  const handleUploadCV = async () => {
    setIsParsing(true);
    try {
      if ((window as any).go) {
        const parsedData = await UploadAndParseCV();
        if (parsedData && parsedData.target_roles) {
          setProfile(parsedData);
          setCvName("Parsed_CV.pdf");
        }
      }
    } catch(e) {
      console.error(e);
    } finally {
      setIsParsing(false);
    }
  };

  const handleStartDaemon = async () => {
    setStatus("Starting Forge Engine...");
    try {
      if ((window as any).go) {
        const success = await StartDaemon(profile);
        if (success) setStatus("✅ Daemon Running");
        else setStatus("❌ Failed to start engine");
      }
    } catch (e) {
      console.error(e);
      setStatus("❌ Critical failure");
    }
  };

  return (
    <div className="p-12 max-w-7xl w-full mx-auto space-y-12 overflow-y-auto">
      {/* Cabeçalho da página */}
      <header className="space-y-2">
        <h1 className="text-4xl font-bold tracking-tight text-zinc-950 font-headline">Base Profile & Target Directives</h1>
        <p className="text-on-surface-variant font-body">Define your core identity and automated application constraints.</p>
      </header>

      {/* Conteúdo principal */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8 items-start">
        {/* Painel esquerdo: perfil e CV */}
        <section className="lg:col-span-5 space-y-6">
          <div className="bg-surface-container-lowest border border-outline-variant/30 rounded-lg p-8 shadow-sm">
            <div className="flex items-center gap-3 mb-8">
              <span className="material-symbols-outlined text-primary" data-icon="cloud_upload">cloud_upload</span>
              <h2 className="text-lg font-semibold font-headline">Source Documentation</h2>
            </div>
            
            <div 
              onClick={handleUploadCV}
              className={`border-2 border-dashed border-outline-variant/50 rounded-lg p-10 flex flex-col items-center justify-center text-center space-y-4 hover:border-primary/50 hover:bg-primary/5 transition-all cursor-pointer group ${isParsing ? 'opacity-50 pointer-events-none' : ''}`}
            >
              <div className="w-12 h-12 rounded-full bg-surface-container-low flex items-center justify-center text-on-surface-variant group-hover:text-primary transition-colors">
                <span className={`material-symbols-outlined ${isParsing ? 'animate-spin' : ''}`}>{isParsing ? 'sync' : 'add'}</span>
              </div>
              <div className="space-y-1">
                <p className="font-medium text-sm">{isParsing ? 'Parsing document with AI...' : 'Select your base CV (PDF)'}</p>
                <p className="text-xs text-on-surface-variant">Max file size: 10MB</p>
              </div>
            </div>
            
            {cvName && (
              <div className="mt-8 space-y-3">
                <div className="flex items-center justify-between p-4 bg-surface-container-low rounded-lg border border-outline-variant/10">
                  <div className="flex items-center gap-3">
                    <span className="material-symbols-outlined text-primary" data-icon="description">description</span>
                    <div className="flex flex-col">
                      <span className="text-sm font-mono font-medium">{cvName}</span>
                      <span className="text-[10px] text-zinc-400 uppercase tracking-widest">AI mapped • 100%</span>
                    </div>
                  </div>
                  <button onClick={() => setCvName("")} className="text-on-surface-variant hover:text-error transition-colors">
                    <span className="material-symbols-outlined text-xl" data-icon="delete">delete</span>
                  </button>
                </div>
              </div>
            )}
          </div>

          <div className="bg-surface-container-highest/30 rounded-lg p-6 border border-outline-variant/10">
            <h3 className="text-xs font-bold uppercase tracking-widest text-on-surface-variant mb-4">Metadata Insight</h3>
            <div className="space-y-4">
              <div className="flex justify-between items-center">
                <span className="text-xs font-mono">Last Indexed</span>
                <span className="text-xs font-mono text-primary">{cvName ? 'Today' : 'Never'}</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-xs font-mono">Keyword Extraction</span>
                <span className={`text-xs font-bold tracking-widest ${cvName ? 'text-green-600' : 'text-zinc-400'}`}>{cvName ? 'COMPLETED' : 'PENDING CV'}</span>
              </div>
            </div>
          </div>
        </section>

        {/* Painel direito: regras de automação */}
        <section className="lg:col-span-7 space-y-6">
          <div className="bg-surface-container-lowest border border-outline-variant/30 rounded-lg p-8 shadow-sm">
            <div className="flex items-center gap-3 mb-8">
              <span className="material-symbols-outlined text-primary" data-icon="bolt">bolt</span>
              <h2 className="text-lg font-semibold font-headline">Automation Directives</h2>
            </div>
            
            <form className="space-y-10" onSubmit={e => e.preventDefault()}>
              {/* Funções alvo */}
              <div className="space-y-4">
                <div className="flex justify-between items-end">
                   <label className="text-xs font-bold uppercase tracking-widest text-on-surface-variant">Target Roles</label>
                   <span className="text-[10px] text-zinc-400 font-mono">{profile.target_roles.length} selected roles</span>
                </div>
                
                {/* Campo visual para as funções alvo */}
                <div className="flex flex-wrap gap-2 p-3 bg-surface-container-low border border-outline-variant/20 rounded-xl min-h-[56px] focus-within:border-primary/50 focus-within:ring-1 focus-within:ring-primary/50 transition-all">
                  {profile.target_roles.map(r => (
                    <span key={r} className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-white shadow-sm border border-outline-variant/50 rounded-full text-xs font-semibold animate-in fade-in zoom-in duration-200">
                      {r}
                      <span className="material-symbols-outlined text-[14px] text-zinc-400 hover:text-error cursor-pointer transition-colors" onClick={() => setProfile({...profile, target_roles: profile.target_roles.filter(x => x !== r)})}>cancel</span>
                    </span>
                  ))}
                  <input className="bg-transparent border-none outline-none focus:ring-0 text-sm flex-1 min-w-[120px]" placeholder="Type & press Enter..." list="roles-suggestions" onKeyDown={e => {
                    if (e.key === 'Enter' && e.currentTarget.value.trim()) {
                      const val = e.currentTarget.value.trim();
                      if(!profile.target_roles.includes(val)) setProfile({...profile, target_roles: [...profile.target_roles, val]});
                      e.currentTarget.value = '';
                    }
                  }}/>
                  <datalist id="roles-suggestions">
                     {["Software Engineer", "Backend Developer", "Frontend Developer", "Fullstack Developer", "DevOps Engineer", "Cloud Architect"].map(r => <option key={r} value={r}/>)}
                  </datalist>
                </div>

                {/* Sugestões rápidas */}
                <div className="flex flex-wrap gap-2 mt-2">
                   {["Backend Engineer", "Tech Lead", "Data Engineer"].filter(r => !profile.target_roles.includes(r)).map(sug => (
                      <button type="button" key={sug} onClick={() => setProfile({...profile, target_roles: [...profile.target_roles, sug]})} className="inline-flex items-center gap-1 px-3 py-1 bg-zinc-50 hover:bg-zinc-100 border border-dashed border-zinc-300 rounded-full text-[11px] text-zinc-500 font-medium transition-colors">
                         <span className="material-symbols-outlined text-[12px]">add</span> {sug}
                      </button>
                   ))}
                </div>
              </div>

              {/* Stack principal */}
              <div className="space-y-4">
                <div className="flex justify-between items-end">
                   <label className="text-xs font-bold uppercase tracking-widest text-on-surface-variant">Core Stack</label>
                   <span className="text-[10px] text-zinc-400 font-mono">{profile.core_stack.length} technologies</span>
                </div>
                
                <div className="flex flex-wrap gap-2 p-3 bg-surface-container-low border border-outline-variant/20 rounded-xl min-h-[56px] focus-within:border-primary/50 focus-within:ring-1 focus-within:ring-primary/50 transition-all">
                  {profile.core_stack.map(s => (
                    <span key={s} className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-zinc-900 border border-zinc-950 shadow-sm text-white rounded-full text-xs font-semibold animate-in fade-in zoom-in duration-200">
                      {s}
                      <span className="material-symbols-outlined text-[14px] text-zinc-400 hover:text-white cursor-pointer transition-colors" onClick={() => setProfile({...profile, core_stack: profile.core_stack.filter(x => x !== s)})}>cancel</span>
                    </span>
                  ))}
                  <input className="bg-transparent border-none outline-none focus:ring-0 text-sm flex-1 min-w-[120px]" placeholder="Add languages, frameworks..." list="stack-suggestions" onKeyDown={e => {
                    if (e.key === 'Enter' && e.currentTarget.value.trim()) {
                      const val = e.currentTarget.value.trim();
                      if(!profile.core_stack.includes(val)) setProfile({...profile, core_stack: [...profile.core_stack, val]});
                      e.currentTarget.value = '';
                    }
                  }}/>
                  <datalist id="stack-suggestions">
                     {["JavaScript", "TypeScript", "Python", "Java", "Go", "Rust", "C#", "React", "Node.js", "AWS", "Docker", "Kubernetes"].map(r => <option key={r} value={r}/>)}
                  </datalist>
                </div>

                <div className="flex flex-wrap gap-2 mt-2">
                   {["Go", "Rust", "React", "Docker", "AWS"].filter(r => !profile.core_stack.includes(r)).map(sug => (
                      <button type="button" key={sug} onClick={() => setProfile({...profile, core_stack: [...profile.core_stack, sug]})} className="inline-flex items-center gap-1 px-3 py-1 bg-indigo-50 hover:bg-indigo-100 border border-dashed border-indigo-200 rounded-full text-[11px] text-indigo-600 font-medium transition-colors">
                         <span className="material-symbols-outlined text-[12px]">add</span> {sug}
                      </button>
                   ))}
                </div>
              </div>

              <hr className="border-outline-variant/10" />

              {/* Variáveis fixas */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                
                {/* Salário mínimo */}
                <div className="space-y-3">
                   <label className="text-xs font-bold uppercase tracking-widest text-on-surface-variant flex items-center gap-2">
                     <span className="material-symbols-outlined text-[14px]">payments</span> Min Salary Floor
                   </label>
                   <div className="relative flex items-center">
                      <span className="absolute left-4 text-zinc-400 font-bold">$</span>
                      <input 
                        type="text"
                        value={profile.min_salary_floor} 
                        onChange={e => {
                          // Aceita só números e formata como moeda.
                          const rawNum = e.target.value.replace(/\D/g, '');
                          const formatted = rawNum ? new Intl.NumberFormat('en-US').format(Number(rawNum)) : '';
                          setProfile({...profile, min_salary_floor: formatted});
                        }} 
                        className="w-full pl-8 pr-4 py-4 bg-surface-container-low border border-outline-variant/20 rounded-xl font-mono text-lg font-bold focus:border-green-500 focus:ring-1 focus:ring-green-500 transition-all outline-none" placeholder="120,000" 
                      />
                   </div>
                   <p className="text-[10px] text-zinc-400">Daemon skips jobs falling below this minimal bar.</p>
                </div>

                {/* Velocidade máxima de candidaturas */}
                <div className="space-y-3">
                   <div className="flex justify-between items-end">
                       <label className="text-xs font-bold uppercase tracking-widest text-on-surface-variant flex items-center gap-2">
                         <span className="material-symbols-outlined text-[14px]">speed</span> Max Velocity (Apps/Day)
                       </label>
                       <span className="text-2xl font-black font-mono text-zinc-900 leading-none">{profile.apps_per_day}</span>
                   </div>
                   
                   <div className="pt-4 pb-2">
                      <input 
                        type="range" 
                        min="1" 
                        max="200" 
                        value={profile.apps_per_day} 
                        onChange={e => setProfile({...profile, apps_per_day: parseInt(e.target.value)})} 
                        className="w-full h-2 bg-surface-container-highest rounded-lg appearance-none cursor-pointer accent-primary" 
                      />
                   </div>
                   <div className="flex justify-between text-[10px] font-bold text-zinc-400 font-mono">
                      <span>1</span>
                      <span className="text-error">200 LIMIT</span>
                   </div>
                </div>
              </div>

              {/* Alternância de remoto */}
              <div className="p-4 bg-blue-50/50 border border-blue-100 rounded-xl flex items-center justify-between">
                <div className="space-y-1">
                  <p className="font-semibold text-sm text-blue-950 flex items-center gap-2">
                    Strictly 100% Remote 
                    <span className="material-symbols-outlined text-blue-600 text-[16px]">public</span>
                  </p>
                  <p className="text-[11px] text-blue-800/70 font-medium">Filter out all hybrid and on-site opportunities automatically.</p>
                </div>
                <button 
                   onClick={() => setProfile({...profile, strictly_remote: !profile.strictly_remote})} 
                   type="button" 
                   className={`relative inline-flex h-7 w-12 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-300 ease-in-out focus:outline-none shadow-inner ${profile.strictly_remote ? 'bg-blue-600' : 'bg-surface-variant'}`}
                >
                  <span className={`pointer-events-none inline-block h-6 w-6 transform rounded-full bg-white shadow-md ring-0 transition duration-300 ease-in-out ${profile.strictly_remote ? 'translate-x-5' : 'translate-x-0'}`}></span>
                </button>
              </div>

            </form>
          </div>
        </section>
      </div>

      {/* Ação principal */}
      <footer className="flex justify-between items-center pt-8 border-t border-outline-variant/20">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <div className={`w-2 h-2 rounded-full ${status.includes('❌') ? 'bg-error' : 'bg-green-500 animate-pulse'}`}></div>
            <span className="text-[10px] font-mono text-on-surface-variant uppercase">{status}</span>
          </div>
        </div>
        <button onClick={handleStartDaemon} className="flex items-center gap-3 px-8 py-4 bg-zinc-950 text-white rounded-lg hover:bg-zinc-800 transition-all shadow-xl shadow-zinc-200 group">
          <span className="font-semibold text-sm">Save & Start Daemon</span>
          <span className="material-symbols-outlined group-hover:translate-x-1 transition-transform" data-icon="play_arrow">play_arrow</span>
        </button>
      </footer>
    </div>
  );
}
