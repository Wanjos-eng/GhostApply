import { useState, useEffect } from 'react';
import { LoadSettings, SaveSettings, VerifyIMAP } from "../../wailsjs/go/main/App";

export function Settings() {
  const [cfg, setCfg] = useState({
    cohere_api_key: "",
    groq_api_key: "",
    gemini_api_key: "",
    ats_min_score: "0.40",
    search_keywords: "software engineer,backend,java,golang",
    search_country: "BR",
    gupy_company_urls: "",
    greenhouse_boards: "",
    lever_companies: "",
    imap_server: "",
    imap_user: "",
    imap_pass: ""
  });
  const [status, setStatus] = useState("");
  const [imapTest, setImapTest] = useState("");
  const [linkedinStatus, setLinkedinStatus] = useState("");
  const [linking, setLinking] = useState(false);

  const notifySystemStatusRefresh = () => {
    window.dispatchEvent(new Event('ghostapply:settings-saved'));
  };

  useEffect(() => {
    // Funciona apenas dentro do runtime do Wails.
    if ((window as any).go) {
      LoadSettings().then(data => {
        if (data) setCfg(prev => ({ ...prev, ...(data as any) }));
      });
    }
  }, []);

  const handleSave = async (e: any) => {
    e.preventDefault();
    setStatus("Saving...");
    if ((window as any).go) {
      const success = await SaveSettings(cfg);
      setStatus(success ? "Settings Saved!" : "Failed to save.");
      if (success) {
        notifySystemStatusRefresh();
      }
      setTimeout(() => setStatus(""), 3000);
    }
  };

  const handleTestImap = async () => {
    setImapTest("Connecting...");
    try {
      if ((window as any).go) {
        const result = await VerifyIMAP(cfg);
        setImapTest(result ? "✅ IMAP connection successful" : "❌ Connection or login failed");
        notifySystemStatusRefresh();
      }
    } catch {
      setImapTest("❌ Fatal error");
    }
  };

  const handleConnectLinkedIn = async () => {
    setLinking(true);
    setLinkedinStatus("Abrindo navegador para login no LinkedIn...");
    try {
      const app = (window as any).go?.main?.App;
      if (!app?.ConnectLinkedInSession) {
        setLinkedinStatus("Runtime Wails indisponível para conectar LinkedIn.");
        return;
      }
      const result = await app.ConnectLinkedInSession();
      setLinkedinStatus(String(result || "Conexão LinkedIn finalizada."));
    } catch (e) {
      setLinkedinStatus(`Falha ao conectar LinkedIn: ${String(e)}`);
    } finally {
      setLinking(false);
    }
  };

  return (
    <div className="w-full h-full p-8 space-y-8">
      <div className="flex flex-col gap-2">
        <h1 className="font-headline font-bold text-[3.5rem] leading-tight text-on-surface tracking-tight">System Settings</h1>
        <p className="text-on-surface-variant max-w-xl">Core GhostApply API keys and IMAP configuration.</p>
      </div>

      <form onSubmit={handleSave} className="bg-white p-8 rounded shadow-[0px_4px_20px_rgba(0,0,0,0.04)] space-y-6">
        
        {/* API Keys */}
        <div className="space-y-4">
          <h2 className="text-sm font-bold uppercase tracking-widest text-on-surface-variant border-b pb-2">LLM Engine Keys</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="space-y-1">
              <label className="text-xs font-semibold text-zinc-600">Gemini API Key</label>
              <input type="password" value={cfg.gemini_api_key} onChange={e => setCfg({...cfg, gemini_api_key: e.target.value})} className="w-full text-sm p-3 bg-surface-container-low border border-outline-variant/30 rounded focus:ring-primary focus:border-primary" placeholder="AI Studio key..." />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold text-zinc-600">Cohere API Key</label>
              <input type="password" value={cfg.cohere_api_key} onChange={e => setCfg({...cfg, cohere_api_key: e.target.value})} className="w-full text-sm p-3 bg-surface-container-low border border-outline-variant/30 rounded focus:ring-primary focus:border-primary" placeholder="Cohere key..." />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold text-zinc-600">Groq API Key</label>
              <input type="password" value={cfg.groq_api_key} onChange={e => setCfg({...cfg, groq_api_key: e.target.value})} className="w-full text-sm p-3 bg-surface-container-low border border-outline-variant/30 rounded focus:ring-primary focus:border-primary" placeholder="Groq key..." />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold text-zinc-600">ATS Minimum Score (0.00-1.00)</label>
              <input
                type="number"
                min="0"
                max="1"
                step="0.01"
                value={cfg.ats_min_score}
                onChange={e => setCfg({...cfg, ats_min_score: e.target.value})}
                className="w-full text-sm p-3 bg-surface-container-low border border-outline-variant/30 rounded font-mono"
                placeholder="0.40"
              />
              <p className="text-[11px] text-zinc-500">Sugestão: 0.35 (Jr), 0.45 (Pleno), 0.55 (Sênior/Staff).</p>
            </div>
          </div>
        </div>

        <div className="space-y-4 pt-6">
          <h2 className="text-sm font-bold uppercase tracking-widest text-on-surface-variant border-b pb-2">Job Providers (Coleta)</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="space-y-1 md:col-span-2">
              <label className="text-xs font-semibold text-zinc-600">Search Keywords (CSV)</label>
              <input
                type="text"
                value={cfg.search_keywords}
                onChange={e => setCfg({...cfg, search_keywords: e.target.value})}
                className="w-full text-sm p-3 bg-surface-container-low border border-outline-variant/30 rounded font-mono"
                placeholder="software engineer,backend,java,golang"
              />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold text-zinc-600">Search Country</label>
              <input
                type="text"
                value={cfg.search_country}
                onChange={e => setCfg({...cfg, search_country: e.target.value})}
                className="w-full text-sm p-3 bg-surface-container-low border border-outline-variant/30 rounded font-mono"
                placeholder="BR"
              />
            </div>
            <div className="space-y-1 md:col-span-2">
              <label className="text-xs font-semibold text-zinc-600">Gupy Boards (CSV URLs)</label>
              <input
                type="text"
                value={cfg.gupy_company_urls}
                onChange={e => setCfg({...cfg, gupy_company_urls: e.target.value})}
                className="w-full text-sm p-3 bg-surface-container-low border border-outline-variant/30 rounded font-mono"
                placeholder="empresa1.gupy.io,empresa2.gupy.io"
              />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold text-zinc-600">Greenhouse Boards (CSV slugs)</label>
              <input
                type="text"
                value={cfg.greenhouse_boards}
                onChange={e => setCfg({...cfg, greenhouse_boards: e.target.value})}
                className="w-full text-sm p-3 bg-surface-container-low border border-outline-variant/30 rounded font-mono"
                placeholder="company-a,company-b"
              />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold text-zinc-600">Lever Companies (CSV slugs)</label>
              <input
                type="text"
                value={cfg.lever_companies}
                onChange={e => setCfg({...cfg, lever_companies: e.target.value})}
                className="w-full text-sm p-3 bg-surface-container-low border border-outline-variant/30 rounded font-mono"
                placeholder="company-a,company-b"
              />
            </div>
          </div>
          <div className="rounded-lg border border-blue-200 bg-blue-50 p-4 space-y-3">
            <div className="flex items-center justify-between gap-4">
              <p className="text-xs font-semibold text-blue-900">
                LinkedIn exige sessão válida para melhor cobertura. Conecte via navegador para gerar o `session.json`.
              </p>
              <button
                type="button"
                onClick={handleConnectLinkedIn}
                disabled={linking}
                className="px-3 py-2 bg-blue-600 hover:bg-blue-500 disabled:bg-blue-300 text-white text-xs font-semibold rounded transition"
              >
                {linking ? 'Conectando...' : 'Conectar LinkedIn'}
              </button>
            </div>
            {linkedinStatus && <p className="text-[11px] text-blue-800 font-mono">{linkedinStatus}</p>}
          </div>
        </div>

        {/* IMAP Configurations */}
        <div className="space-y-4 pt-6">
          <div className="flex items-center justify-between border-b pb-2">
             <h2 className="text-sm font-bold uppercase tracking-widest text-on-surface-variant">IMAP Listener (Recruiter Extraction)</h2>
             <div className="flex items-center gap-4">
                <span className="text-xs font-semibold text-zinc-500">{imapTest}</span>
               <button type="button" onClick={handleTestImap} className="px-3 py-1 bg-surface-container-high hover:bg-surface-dim text-xs font-semibold rounded transition text-blue-800 border border-blue-200">Test Connection</button>
             </div>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="space-y-1 md:col-span-2">
              <label className="text-xs font-semibold text-zinc-600">IMAP Server</label>
              <input type="text" value={cfg.imap_server} onChange={e => setCfg({...cfg, imap_server: e.target.value})} className="w-full text-sm p-3 bg-surface-container-low border border-outline-variant/30 rounded font-mono" placeholder="imap.gmail.com:993" />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold text-zinc-600">Email Address (User)</label>
              <input type="email" value={cfg.imap_user} onChange={e => setCfg({...cfg, imap_user: e.target.value})} className="w-full text-sm p-3 bg-surface-container-low border border-outline-variant/30 rounded" placeholder="you@domain.com" />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold text-zinc-600">App-Specific Password</label>
              <input type="password" value={cfg.imap_pass} onChange={e => setCfg({...cfg, imap_pass: e.target.value})} className="w-full text-sm p-3 bg-surface-container-low border border-outline-variant/30 rounded" placeholder="abcd efgh ijkl mnop" />
            </div>
          </div>
        </div>

        {/* Actions */}
        <div className="pt-6 flex justify-between items-center px-2">
          <span className="text-sm font-semibold text-green-600">{status}</span>
          <button type="submit" className="px-6 py-3 bg-zinc-900 text-white rounded font-semibold hover:bg-zinc-800 transition shadow">
            Save Configurations
          </button>
        </div>

      </form>
    </div>
  );
}
