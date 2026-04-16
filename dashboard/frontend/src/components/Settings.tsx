import { useState, useEffect } from 'react';
import { LoadSettings, SaveSettings, VerifyIMAP } from "../../wailsjs/go/main/App";

type SystemStatus = {
  database: string;
  database_detail: string;
  database_path: string;
  cohere: string;
  cohere_detail: string;
  groq: string;
  groq_detail: string;
  gemini: string;
  gemini_detail: string;
  imap: string;
  imap_detail: string;
};

const defaultSystemStatus: SystemStatus = {
  database: "...",
  database_detail: "Aguardando diagnóstico...",
  database_path: "",
  cohere: "...",
  cohere_detail: "",
  groq: "...",
  groq_detail: "",
  gemini: "...",
  gemini_detail: "",
  imap: "...",
  imap_detail: "",
};

function normalizeSystemStatus(raw: unknown): SystemStatus {
  if (!raw || typeof raw !== 'object') {
    return defaultSystemStatus;
  }

  const obj = raw as Record<string, unknown>;
  const pick = (key: keyof SystemStatus) => {
    const value = obj[key];
    return typeof value === 'string' ? value : defaultSystemStatus[key];
  };

  return {
    database: pick('database'),
    database_detail: pick('database_detail'),
    database_path: pick('database_path'),
    cohere: pick('cohere'),
    cohere_detail: pick('cohere_detail'),
    groq: pick('groq'),
    groq_detail: pick('groq_detail'),
    gemini: pick('gemini'),
    gemini_detail: pick('gemini_detail'),
    imap: pick('imap'),
    imap_detail: pick('imap_detail'),
  };
}

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
  const [systemStatus, setSystemStatus] = useState<SystemStatus>(defaultSystemStatus);
  const [keyActionBusy, setKeyActionBusy] = useState<{ gemini: boolean; cohere: boolean; groq: boolean }>({
    gemini: false,
    cohere: false,
    groq: false,
  });

  const notifySystemStatusRefresh = () => {
    window.dispatchEvent(new Event('ghostapply:settings-saved'));
  };

  const fetchSystemStatus = async () => {
    try {
      const app = (window as any).go?.main?.App;
      if (!app?.GetSystemStatus) {
        return;
      }
      const snapshot = await app.GetSystemStatus();
      setSystemStatus(normalizeSystemStatus(snapshot));
    } catch (err) {
      console.error('Failed to load system status:', err);
      setSystemStatus(defaultSystemStatus);
    }
  };

  useEffect(() => {
    // Funciona apenas dentro do runtime do Wails.
    if ((window as any).go) {
      LoadSettings().then(data => {
        if (data) setCfg(prev => ({ ...prev, ...(data as any) }));
      });
      fetchSystemStatus();
    }

    const interval = setInterval(fetchSystemStatus, 15000);
    const onRefresh = () => {
      fetchSystemStatus();
    };
    window.addEventListener('ghostapply:settings-saved', onRefresh);

    return () => {
      clearInterval(interval);
      window.removeEventListener('ghostapply:settings-saved', onRefresh);
    };
  }, []);

  const handleSave = async (e: any) => {
    e.preventDefault();
    setStatus("Saving...");
    if ((window as any).go) {
      const success = await SaveSettings(cfg);
      setStatus(success ? "Settings Saved!" : "Failed to save.");
      if (success) {
        notifySystemStatusRefresh();
        await fetchSystemStatus();
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
        await fetchSystemStatus();
      }
    } catch {
      setImapTest("❌ Fatal error");
    }
  };

  const fieldValueForService = (service: 'gemini' | 'cohere' | 'groq') => {
    if (service === 'gemini') return cfg.gemini_api_key;
    if (service === 'cohere') return cfg.cohere_api_key;
    return cfg.groq_api_key;
  };

  const applySingleServiceStatus = (service: 'gemini' | 'cohere' | 'groq', serviceStatus: string, detail: string) => {
    setSystemStatus(prev => ({
      ...prev,
      [service]: serviceStatus,
      [`${service}_detail`]: detail,
    } as SystemStatus));
  };

  const handleValidateSingleKey = async (service: 'gemini' | 'cohere' | 'groq') => {
    setKeyActionBusy(prev => ({ ...prev, [service]: true }));
    try {
      const app = (window as any).go?.main?.App;
      if (!app?.VerifySingleCredential) {
        setStatus("Runtime não expôs VerifySingleCredential ainda. Reinicie o Wails.");
        return;
      }

      const currentValue = fieldValueForService(service);
      const result = await app.VerifySingleCredential(service, currentValue);
      const state = typeof result?.status === 'string' ? result.status : '✗ ERRO';
      const detail = typeof result?.detail === 'string' ? result.detail : 'validação sem detalhe';
      applySingleServiceStatus(service, state, detail);
      setStatus(`${service.toUpperCase()}: ${state} - ${detail}`);
      notifySystemStatusRefresh();
    } catch (err) {
      setStatus(`Falha ao validar ${service}: ${String(err)}`);
    } finally {
      setKeyActionBusy(prev => ({ ...prev, [service]: false }));
      setTimeout(() => setStatus(""), 3500);
    }
  };

  const handleClearSingleKey = async (service: 'gemini' | 'cohere' | 'groq') => {
    setKeyActionBusy(prev => ({ ...prev, [service]: true }));
    try {
      const app = (window as any).go?.main?.App;
      if (!app?.ClearPersistedSecret) {
        setStatus("Runtime não expôs ClearPersistedSecret ainda. Reinicie o Wails.");
        return;
      }

      const result = await app.ClearPersistedSecret(service);
      if (service === 'gemini') {
        setCfg(prev => ({ ...prev, gemini_api_key: "" }));
      } else if (service === 'cohere') {
        setCfg(prev => ({ ...prev, cohere_api_key: "" }));
      } else {
        setCfg(prev => ({ ...prev, groq_api_key: "" }));
      }

      applySingleServiceStatus(service, '⚠ CHAVE AUSENTE', 'chave limpa localmente; salve uma nova para validar');
      setStatus(String(result || `${service.toUpperCase()} limpa com sucesso.`));
      notifySystemStatusRefresh();
      await fetchSystemStatus();
    } catch (err) {
      setStatus(`Falha ao limpar ${service}: ${String(err)}`);
    } finally {
      setKeyActionBusy(prev => ({ ...prev, [service]: false }));
      setTimeout(() => setStatus(""), 3500);
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
              <div className="flex items-center gap-2">
                <input type="password" value={cfg.gemini_api_key} onChange={e => setCfg({...cfg, gemini_api_key: e.target.value})} className="w-full text-sm p-3 bg-surface-container-low border border-outline-variant/30 rounded focus:ring-primary focus:border-primary" placeholder="AI Studio key..." />
                <button type="button" onClick={() => handleValidateSingleKey('gemini')} disabled={keyActionBusy.gemini} className="px-3 py-2 rounded border border-blue-200 text-blue-700 bg-blue-50 hover:bg-blue-100 text-xs font-semibold disabled:opacity-50" title="Validar apenas Gemini">↻</button>
                <button type="button" onClick={() => handleClearSingleKey('gemini')} disabled={keyActionBusy.gemini} className="px-3 py-2 rounded border border-rose-200 text-rose-700 bg-rose-50 hover:bg-rose-100 text-xs font-semibold disabled:opacity-50" title="Limpar apenas Gemini persistido">✕</button>
              </div>
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold text-zinc-600">Cohere API Key</label>
              <div className="flex items-center gap-2">
                <input type="password" value={cfg.cohere_api_key} onChange={e => setCfg({...cfg, cohere_api_key: e.target.value})} className="w-full text-sm p-3 bg-surface-container-low border border-outline-variant/30 rounded focus:ring-primary focus:border-primary" placeholder="Cohere key..." />
                <button type="button" onClick={() => handleValidateSingleKey('cohere')} disabled={keyActionBusy.cohere} className="px-3 py-2 rounded border border-blue-200 text-blue-700 bg-blue-50 hover:bg-blue-100 text-xs font-semibold disabled:opacity-50" title="Validar apenas Cohere">↻</button>
                <button type="button" onClick={() => handleClearSingleKey('cohere')} disabled={keyActionBusy.cohere} className="px-3 py-2 rounded border border-rose-200 text-rose-700 bg-rose-50 hover:bg-rose-100 text-xs font-semibold disabled:opacity-50" title="Limpar apenas Cohere persistido">✕</button>
              </div>
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold text-zinc-600">Groq API Key</label>
              <div className="flex items-center gap-2">
                <input type="password" value={cfg.groq_api_key} onChange={e => setCfg({...cfg, groq_api_key: e.target.value})} className="w-full text-sm p-3 bg-surface-container-low border border-outline-variant/30 rounded focus:ring-primary focus:border-primary" placeholder="Groq key..." />
                <button type="button" onClick={() => handleValidateSingleKey('groq')} disabled={keyActionBusy.groq} className="px-3 py-2 rounded border border-blue-200 text-blue-700 bg-blue-50 hover:bg-blue-100 text-xs font-semibold disabled:opacity-50" title="Validar apenas Groq">↻</button>
                <button type="button" onClick={() => handleClearSingleKey('groq')} disabled={keyActionBusy.groq} className="px-3 py-2 rounded border border-rose-200 text-rose-700 bg-rose-50 hover:bg-rose-100 text-xs font-semibold disabled:opacity-50" title="Limpar apenas Groq persistido">✕</button>
              </div>
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
          <div className="flex items-center gap-2">
            <button type="submit" className="px-6 py-3 bg-zinc-900 text-white rounded font-semibold hover:bg-zinc-800 transition shadow">
              Save Configurations
            </button>
          </div>
        </div>

      </form>

      <section className="bg-white p-8 rounded shadow-[0px_4px_20px_rgba(0,0,0,0.04)] space-y-6">
        <div className="flex items-center justify-between border-b pb-3">
          <h2 className="text-sm font-bold uppercase tracking-widest text-on-surface-variant">System Status Live</h2>
          <button
            type="button"
            onClick={fetchSystemStatus}
            className="px-3 py-1.5 bg-surface-container-high hover:bg-surface-dim text-xs font-semibold rounded transition"
          >
            Refresh Now
          </button>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="rounded-lg border border-zinc-200 p-4 bg-zinc-50">
            <p className="text-xs uppercase tracking-wider text-zinc-500 mb-1">Database</p>
            <p className="text-sm font-bold text-zinc-900">{systemStatus.database}</p>
            <p className="text-xs text-zinc-600 mt-1">{systemStatus.database_detail}</p>
            {systemStatus.database_path && <p className="text-[11px] font-mono text-zinc-500 mt-2 break-all">{systemStatus.database_path}</p>}
          </div>

          <div className="rounded-lg border border-zinc-200 p-4 bg-zinc-50">
            <p className="text-xs uppercase tracking-wider text-zinc-500 mb-1">IMAP</p>
            <p className="text-sm font-bold text-zinc-900">{systemStatus.imap}</p>
            <p className="text-xs text-zinc-600 mt-1">{systemStatus.imap_detail}</p>
          </div>

          <div className="rounded-lg border border-zinc-200 p-4 bg-zinc-50">
            <p className="text-xs uppercase tracking-wider text-zinc-500 mb-1">Gemini</p>
            <p className="text-sm font-bold text-zinc-900">{systemStatus.gemini}</p>
            <p className="text-xs text-zinc-600 mt-1">{systemStatus.gemini_detail}</p>
          </div>

          <div className="rounded-lg border border-zinc-200 p-4 bg-zinc-50">
            <p className="text-xs uppercase tracking-wider text-zinc-500 mb-1">Cohere</p>
            <p className="text-sm font-bold text-zinc-900">{systemStatus.cohere}</p>
            <p className="text-xs text-zinc-600 mt-1">{systemStatus.cohere_detail}</p>
          </div>

          <div className="rounded-lg border border-zinc-200 p-4 bg-zinc-50 md:col-span-2">
            <p className="text-xs uppercase tracking-wider text-zinc-500 mb-1">Groq</p>
            <p className="text-sm font-bold text-zinc-900">{systemStatus.groq}</p>
            <p className="text-xs text-zinc-600 mt-1">{systemStatus.groq_detail}</p>
          </div>
        </div>
      </section>
    </div>
  );
}
