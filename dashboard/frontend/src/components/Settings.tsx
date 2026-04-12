import { useState, useEffect } from 'react';
import { LoadSettings, SaveSettings, VerifyIMAP } from "../../wailsjs/go/main/App";

export function Settings() {
  const [cfg, setCfg] = useState({
    cohere_api_key: "",
    groq_api_key: "",
    gemini_api_key: "",
    imap_server: "",
    imap_user: "",
    imap_pass: ""
  });
  const [status, setStatus] = useState("");
  const [imapTest, setImapTest] = useState("");

  useEffect(() => {
    // Funciona apenas dentro do runtime do Wails.
    if ((window as any).go) {
      LoadSettings().then(data => {
        if (data) setCfg(data);
      });
    }
  }, []);

  const handleSave = async (e: any) => {
    e.preventDefault();
    setStatus("Saving...");
    if ((window as any).go) {
      const success = await SaveSettings(cfg);
      setStatus(success ? "Settings Saved!" : "Failed to save.");
      setTimeout(() => setStatus(""), 3000);
    }
  };

  const handleTestImap = async () => {
    setImapTest("Connecting...");
    try {
      if ((window as any).go) {
        const result = await VerifyIMAP(cfg);
        setImapTest(result ? "✅ IMAP connection successful" : "❌ Connection or login failed");
      }
    } catch {
      setImapTest("❌ Fatal error");
    }
  };

  return (
    <div className="p-8 space-y-8 overflow-y-auto w-full">
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
