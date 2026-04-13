import { useEffect, useRef, useState } from 'react';
import { GetAutomationPipelineStatus, StartAutomationPipeline, UploadAndParseCV } from "../../wailsjs/go/main/App";

const PROFILE_DRAFT_KEY = 'ghostapply:base-profile-draft';

type ProfileState = {
  target_roles: string[];
  core_stack: string[];
  suggested_keywords: string[];
  suggested_exclude_keywords: string[];
  suggested_seniority: string;
  suggested_remote_policy: string;
  suggested_sources: string[];
  gemini_rationale: string;
  strictly_remote: boolean;
  min_salary_floor: string;
  apps_per_day: number;
  source_file: string;
  parse_status: string;
  parse_error_message: string;
};

type PipelineStep = {
  id: string;
  title: string;
  status: string;
  detail: string;
  started_at: string;
  finished_at: string;
};

type PipelineStatus = {
  state: string;
  summary: string;
  started_at: string;
  updated_at: string;
  finished_at: string;
  steps: PipelineStep[];
  logs: string[];
};

const defaultPipelineStatus: PipelineStatus = {
  state: 'idle',
  summary: 'Pipeline parado',
  started_at: '',
  updated_at: '',
  finished_at: '',
  steps: [],
  logs: [],
};

const defaultProfile: ProfileState = {
  target_roles: [],
  core_stack: [],
  suggested_keywords: [],
  suggested_exclude_keywords: [],
  suggested_seniority: 'any',
  suggested_remote_policy: 'strict-remote',
  suggested_sources: [],
  gemini_rationale: '',
  strictly_remote: true,
  min_salary_floor: "",
  apps_per_day: 50,
  source_file: "",
  parse_status: "idle",
  parse_error_message: "",
};

const clampAppsPerDay = (value: number) => {
  if (!Number.isFinite(value)) return 50;
  return Math.min(200, Math.max(1, Math.trunc(value)));
};

function normalizeProfileInput(raw: any, fallback: ProfileState = defaultProfile): ProfileState {
  const src = raw && typeof raw === 'object' ? raw : {};
  const rawApps = Number(src.apps_per_day);
  const fallbackApps = Number.isFinite(fallback.apps_per_day) ? fallback.apps_per_day : 50;
  const normalizedApps = Number.isFinite(rawApps)
    ? Math.min(200, Math.max(1, Math.trunc(rawApps)))
    : Math.min(200, Math.max(1, Math.trunc(fallbackApps)));
  return {
    target_roles: Array.isArray(src.target_roles) ? src.target_roles.filter((v: unknown) => typeof v === 'string' && v.trim() !== '') : fallback.target_roles,
    core_stack: Array.isArray(src.core_stack) ? src.core_stack.filter((v: unknown) => typeof v === 'string' && v.trim() !== '') : fallback.core_stack,
    suggested_keywords: Array.isArray(src.suggested_keywords) ? src.suggested_keywords.filter((v: unknown) => typeof v === 'string' && v.trim() !== '') : fallback.suggested_keywords,
    suggested_exclude_keywords: Array.isArray(src.suggested_exclude_keywords) ? src.suggested_exclude_keywords.filter((v: unknown) => typeof v === 'string' && v.trim() !== '') : fallback.suggested_exclude_keywords,
    suggested_seniority: typeof src.suggested_seniority === 'string' && src.suggested_seniority.trim() !== '' ? src.suggested_seniority : fallback.suggested_seniority,
    suggested_remote_policy: typeof src.suggested_remote_policy === 'string' && src.suggested_remote_policy.trim() !== '' ? src.suggested_remote_policy : fallback.suggested_remote_policy,
    suggested_sources: Array.isArray(src.suggested_sources) ? src.suggested_sources.filter((v: unknown) => typeof v === 'string' && v.trim() !== '') : fallback.suggested_sources,
    gemini_rationale: typeof src.gemini_rationale === 'string' ? src.gemini_rationale : fallback.gemini_rationale,
    strictly_remote: typeof src.strictly_remote === 'boolean' ? src.strictly_remote : fallback.strictly_remote,
    min_salary_floor: typeof src.min_salary_floor === 'string' ? src.min_salary_floor : fallback.min_salary_floor,
    apps_per_day: normalizedApps,
    source_file: typeof src.source_file === 'string' ? src.source_file : fallback.source_file,
    parse_status: typeof src.parse_status === 'string' ? src.parse_status : fallback.parse_status,
    parse_error_message: typeof src.parse_error_message === 'string' ? src.parse_error_message : fallback.parse_error_message,
  };
}

function loadDraftProfile(): ProfileState {
  try {
    const raw = localStorage.getItem(PROFILE_DRAFT_KEY);
    if (!raw) {
      return defaultProfile;
    }
    return normalizeProfileInput(JSON.parse(raw), defaultProfile);
  } catch {
    return defaultProfile;
  }
}

export function BaseProfile() {
  const [profile, setProfile] = useState<ProfileState>(loadDraftProfile);
  const contentScrollRef = useRef<HTMLDivElement | null>(null);
  const pipelineLogRef = useRef<HTMLDivElement | null>(null);

  const [status, setStatus] = useState("Daemon Ready");
  const [isParsing, setIsParsing] = useState(false);
  const [cvName, setCvName] = useState("");
  const [cvFeedback, setCvFeedback] = useState("");
  const [pipelineStatus, setPipelineStatus] = useState<PipelineStatus>(defaultPipelineStatus);
  const [showPipelinePanel, setShowPipelinePanel] = useState(false);
  const [isStarting, setIsStarting] = useState(false);
  const [lastDraftSavedAt, setLastDraftSavedAt] = useState<string>('');

  const clearGeminiDerivedFields = (prev: ProfileState): ProfileState => ({
    ...prev,
    target_roles: [],
    core_stack: [],
    suggested_keywords: [],
    suggested_exclude_keywords: [],
    suggested_seniority: 'any',
    suggested_remote_policy: 'strict-remote',
    suggested_sources: [],
    gemini_rationale: '',
    parse_error_message: '',
  });

  const hasParsedCV = profile.parse_status === 'parsed' && profile.source_file.trim() !== '';
  const visibleTargetRoles = hasParsedCV ? profile.target_roles : [];
  const visibleCoreStack = hasParsedCV ? profile.core_stack : [];

  const setAppsPerDay = (value: number) => {
    setProfile((prev) => ({ ...prev, apps_per_day: clampAppsPerDay(value) }));
  };

  useEffect(() => {
    localStorage.setItem(PROFILE_DRAFT_KEY, JSON.stringify(profile));
    setLastDraftSavedAt(new Date().toLocaleTimeString());
  }, [profile]);

  useEffect(() => {
    if (profile.source_file && !cvName) {
      setCvName(profile.source_file);
    }
  }, [profile.source_file, cvName]);

  useEffect(() => {
    let mounted = true;
    const syncPipeline = async () => {
      try {
        const snapshot = await GetAutomationPipelineStatus();
        if (mounted && snapshot) {
          setPipelineStatus(snapshot as PipelineStatus);
        }
      } catch (err) {
        console.error('Pipeline status poll failed:', err);
      }
    };

    syncPipeline();
    const interval = setInterval(syncPipeline, 1200);
    return () => {
      mounted = false;
      clearInterval(interval);
    };
  }, []);

  useEffect(() => {
    const stabilizeScroll = () => {
      const el = contentScrollRef.current;
      if (!el) {
        return;
      }

      // Reaplica overflow e limita a posição para evitar "travamento" após maximize/minimize.
      const maxTop = Math.max(0, el.scrollHeight - el.clientHeight);
      if (el.scrollTop > maxTop) {
        el.scrollTop = maxTop;
      }

      el.style.overflowY = 'hidden';
      requestAnimationFrame(() => {
        el.style.overflowY = 'auto';
      });
    };

    const onResize = () => {
      requestAnimationFrame(stabilizeScroll);
    };

    window.addEventListener('resize', onResize);
    window.addEventListener('orientationchange', onResize);

    // Primeira estabilização ao montar a tela.
    stabilizeScroll();

    return () => {
      window.removeEventListener('resize', onResize);
      window.removeEventListener('orientationchange', onResize);
    };
  }, []);

  useEffect(() => {
    const logEl = pipelineLogRef.current;
    if (!logEl) {
      return;
    }
    logEl.scrollTop = logEl.scrollHeight;
  }, [pipelineStatus.logs.length, showPipelinePanel]);

  const handleUploadCV = async () => {
    if (isParsing) {
      return;
    }
    setIsParsing(true);
    setCvFeedback("Parsing CV with Gemini...");
    try {
      if ((window as any).go) {
        const parsedData = await UploadAndParseCV();
        const payload = parsedData as any;
        const roles = Array.isArray(payload?.target_roles) ? payload.target_roles : [];
        const stack = Array.isArray(payload?.core_stack) ? payload.core_stack : [];
        const suggested = Array.isArray(payload?.suggested_keywords) ? payload.suggested_keywords : [];
        const excludes = Array.isArray(payload?.suggested_exclude_keywords) ? payload.suggested_exclude_keywords : [];
        const sourceFile = typeof payload?.source_file === 'string' ? payload.source_file : '';
        const parseStatus = typeof payload?.parse_status === 'string' ? payload.parse_status : '';
        const parseErrorMessage = typeof payload?.parse_error_message === 'string' ? payload.parse_error_message : '';

        if (sourceFile) {
          setCvName(sourceFile);
        }

        if (parseStatus === 'parsed') {
          setProfile((prev) => normalizeProfileInput(payload, prev));
          setCvFeedback(`Gemini mapeou o CV: ${roles.length} roles, ${stack.length} stacks, ${suggested.length} keywords e ${excludes.length} exclusões.`);
        } else if (sourceFile && parseStatus === 'uploaded') {
          setProfile((prev) => ({ ...clearGeminiDerivedFields(prev), source_file: sourceFile, parse_status: parseStatus, parse_error_message: parseErrorMessage }));
          setCvFeedback('PDF carregado com sucesso. Parsing IA não executado (verifique GEMINI_API_KEY).');
        } else if (parseStatus === 'cancelled') {
          setCvFeedback('Upload cancelado.');
        } else {
          setProfile((prev) => ({ ...clearGeminiDerivedFields(prev), parse_status: 'error', parse_error_message: parseErrorMessage }));
          setCvFeedback(parseErrorMessage || "Não foi possível processar o PDF selecionado.");
        }
      }
    } catch(e) {
      console.error(e);
      setProfile((prev) => ({ ...clearGeminiDerivedFields(prev), parse_status: 'error', parse_error_message: 'Falha inesperada na análise do PDF.' }));
      setCvFeedback("Failed to parse PDF. Please verify Gemini API key and try again.");
    } finally {
      setIsParsing(false);
    }
  };

  const handleStartDaemon = async () => {
    if (isStarting || pipelineStatus.state === 'running') {
      setStatus("⏳ Pipeline já está em execução");
      setShowPipelinePanel(true);
      return;
    }

    if (!profile.source_file) {
      setStatus("❌ Faça upload do CV antes de iniciar");
      return;
    }
    if (profile.parse_status !== 'parsed') {
      setStatus("❌ A análise do CV pelo Gemini precisa concluir antes de iniciar");
      return;
    }

    setStatus("Starting automation pipeline...");
    setShowPipelinePanel(true);
    setIsStarting(true);
    try {
      if ((window as any).go) {
        const success = await StartAutomationPipeline(profile);
        if (success) {
          setStatus("✅ Pipeline iniciado. Acompanhe a execução em tempo real.");
        } else {
          setStatus("❌ Não foi possível iniciar o pipeline agora.");
        }
      }
    } catch (e) {
      console.error(e);
      setStatus("❌ Critical failure");
    } finally {
      setIsStarting(false);
    }
  };

  return (
    <div className="w-full h-full flex flex-col overflow-hidden p-8">
      {/* Cabeçalho da página */}
      <header className="space-y-2 shrink-0">
        <h1 className="text-4xl font-bold tracking-tight text-zinc-950 font-headline">Base Profile & Target Directives</h1>
        <p className="text-on-surface-variant font-body">Define your core identity and automated application constraints.</p>
      </header>

      {/* Conteúdo principal */}
      <div ref={contentScrollRef} className="grid grid-cols-1 lg:grid-cols-12 gap-8 items-start flex-1 min-h-0 overflow-y-auto pr-1 pb-6 mt-8">
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
                {cvFeedback && <p className="text-xs text-blue-700">{cvFeedback}</p>}
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
                  <button
                    onClick={() => {
                      setCvName("");
                      setCvFeedback("PDF removido.");
                      setProfile((prev) => ({ ...clearGeminiDerivedFields(prev), source_file: '', parse_status: 'idle', parse_error_message: '' }));
                    }}
                    className="text-on-surface-variant hover:text-error transition-colors"
                  >
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

          <div className="bg-white rounded-lg p-6 border border-outline-variant/20 shadow-sm">
            <h3 className="text-xs font-bold uppercase tracking-widest text-on-surface-variant mb-4">Gemini Recommendations</h3>
            {profile.parse_status === 'parsed' ? (
              <div className="space-y-4">
                <div>
                  <p className="text-[11px] uppercase tracking-widest text-zinc-500 mb-2">Suggested Search Keywords</p>
                  <div className="flex flex-wrap gap-2">
                    {profile.suggested_keywords.length === 0 && (
                      <span className="text-xs text-zinc-500">Sem keywords retornadas pelo Gemini.</span>
                    )}
                    {profile.suggested_keywords.map((keyword) => (
                      <span key={keyword} className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-semibold bg-emerald-50 border border-emerald-200 text-emerald-700">
                        {keyword}
                      </span>
                    ))}
                  </div>
                </div>
                <div>
                  <p className="text-[11px] uppercase tracking-widest text-zinc-500 mb-2">Exclude Keywords</p>
                  <div className="flex flex-wrap gap-2">
                    {profile.suggested_exclude_keywords.length === 0 && (
                      <span className="text-xs text-zinc-500">Sem exclusões sugeridas.</span>
                    )}
                    {profile.suggested_exclude_keywords.map((keyword) => (
                      <span key={keyword} className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-semibold bg-rose-50 border border-rose-200 text-rose-700">
                        {keyword}
                      </span>
                    ))}
                  </div>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                  <div className="rounded-lg border border-zinc-200 p-3">
                    <p className="text-[10px] uppercase tracking-widest text-zinc-500 mb-1">Seniority</p>
                    <p className="text-sm font-semibold text-zinc-800">{profile.suggested_seniority}</p>
                  </div>
                  <div className="rounded-lg border border-zinc-200 p-3">
                    <p className="text-[10px] uppercase tracking-widest text-zinc-500 mb-1">Remote Policy</p>
                    <p className="text-sm font-semibold text-zinc-800">{profile.suggested_remote_policy}</p>
                  </div>
                  <div className="rounded-lg border border-zinc-200 p-3">
                    <p className="text-[10px] uppercase tracking-widest text-zinc-500 mb-1">Sources</p>
                    <p className="text-sm font-semibold text-zinc-800">{profile.suggested_sources.length > 0 ? profile.suggested_sources.join(', ') : 'all'}</p>
                  </div>
                </div>
                <div>
                  <p className="text-[11px] uppercase tracking-widest text-zinc-500 mb-2">Rationale</p>
                  <p className="text-sm text-zinc-700 leading-relaxed">
                    {profile.gemini_rationale || 'Gemini não retornou justificativa textual.'}
                  </p>
                </div>
              </div>
            ) : profile.parse_status === 'error' ? (
              <p className="text-sm text-rose-700">{profile.parse_error_message || 'Falha ao analisar o CV com a API.'}</p>
            ) : (
              <p className="text-sm text-zinc-500">As recomendações aparecem automaticamente aqui assim que o CV for inserido e analisado.</p>
            )}
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
                   <span className="text-[10px] text-zinc-400 font-mono">{visibleTargetRoles.length} selected roles</span>
                </div>
                
                {/* Campo visual para as funções alvo */}
                <div className="flex flex-wrap gap-2 p-3 bg-surface-container-low border border-outline-variant/20 rounded-xl min-h-[56px] focus-within:border-primary/50 focus-within:ring-1 focus-within:ring-primary/50 transition-all">
                  {visibleTargetRoles.map(r => (
                    <span key={r} className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-white shadow-sm border border-outline-variant/50 rounded-full text-xs font-semibold animate-in fade-in zoom-in duration-200">
                      {r}
                    </span>
                  ))}
                  {visibleTargetRoles.length === 0 && (
                    <span className="text-xs text-zinc-500">Sem dados. Faça upload do curriculo e aguarde a resposta da API para preencher automaticamente.</span>
                  )}
                </div>
                <p className="text-[11px] text-zinc-500">Preenchido automaticamente pelo Gemini ao inserir o curriculo.</p>
              </div>

              {/* Stack principal */}
              <div className="space-y-4">
                <div className="flex justify-between items-end">
                   <label className="text-xs font-bold uppercase tracking-widest text-on-surface-variant">Core Stack</label>
                   <span className="text-[10px] text-zinc-400 font-mono">{visibleCoreStack.length} technologies</span>
                </div>
                
                <div className="flex flex-wrap gap-2 p-3 bg-surface-container-low border border-outline-variant/20 rounded-xl min-h-[56px] focus-within:border-primary/50 focus-within:ring-1 focus-within:ring-primary/50 transition-all">
                  {visibleCoreStack.map(s => (
                    <span key={s} className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-zinc-900 border border-zinc-950 shadow-sm text-white rounded-full text-xs font-semibold animate-in fade-in zoom-in duration-200">
                      {s}
                    </span>
                  ))}
                  {visibleCoreStack.length === 0 && (
                    <span className="text-xs text-zinc-500">Sem dados. O Core Stack aparece somente após análise valida da API.</span>
                  )}
                </div>
                <p className="text-[11px] text-zinc-500">Gerado automaticamente a partir da analise do CV feita pelo Gemini.</p>
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
                   
                   <div className="pt-4 pb-2 space-y-3">
                      <input 
                        type="range" 
                        min="1" 
                        max="200" 
                        value={profile.apps_per_day} 
                        onChange={e => setAppsPerDay(Number(e.target.value))}
                        className="w-full h-2 bg-surface-container-highest rounded-lg appearance-none cursor-pointer accent-primary" 
                      />
                      <div className="flex items-center gap-2">
                        <button
                          type="button"
                          onClick={() => setAppsPerDay(profile.apps_per_day - 1)}
                          className="px-3 py-1.5 rounded-lg border border-zinc-200 bg-white text-zinc-700 text-xs font-bold"
                        >
                          -1
                        </button>
                        <input
                          type="number"
                          min={1}
                          max={200}
                          value={profile.apps_per_day}
                          onChange={(e) => setAppsPerDay(Number(e.target.value))}
                          className="w-24 px-3 py-1.5 rounded-lg border border-zinc-200 bg-white text-zinc-900 text-sm font-mono"
                        />
                        <button
                          type="button"
                          onClick={() => setAppsPerDay(profile.apps_per_day + 1)}
                          className="px-3 py-1.5 rounded-lg border border-zinc-200 bg-white text-zinc-700 text-xs font-bold"
                        >
                          +1
                        </button>
                      </div>
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
      <footer className="flex justify-between items-center pt-6 border-t border-outline-variant/20 shrink-0 bg-[#f9f9f9]">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <div className={`w-2 h-2 rounded-full ${status.includes('❌') ? 'bg-error' : 'bg-green-500 animate-pulse'}`}></div>
            <span className="text-[10px] font-mono text-on-surface-variant uppercase">{status}</span>
          </div>
          <div className="text-[10px] font-mono text-zinc-500 uppercase">
            {lastDraftSavedAt ? `Rascunho salvo ${lastDraftSavedAt}` : 'Rascunho aguardando alterações'}
          </div>
        </div>
        <button
          onClick={handleStartDaemon}
          disabled={isStarting || pipelineStatus.state === 'running'}
          className="flex items-center gap-3 px-8 py-4 bg-zinc-950 text-white rounded-lg hover:bg-zinc-800 disabled:bg-zinc-400 disabled:cursor-not-allowed transition-all shadow-xl shadow-zinc-200 group"
        >
          <span className="font-semibold text-sm">{isStarting ? 'Inicializando...' : pipelineStatus.state === 'running' ? 'Pipeline em execução' : 'Iniciar Coleta + Candidatura'}</span>
          <span className="material-symbols-outlined group-hover:translate-x-1 transition-transform" data-icon="play_arrow">play_arrow</span>
        </button>
      </footer>

      {showPipelinePanel && (
        <div className="fixed inset-0 z-50 bg-black/40 backdrop-blur-[1px] flex items-center justify-center p-6">
          <div className="w-full max-w-4xl bg-white rounded-xl shadow-2xl border border-zinc-200 overflow-hidden">
            <div className="flex items-center justify-between px-6 py-4 border-b border-zinc-100">
              <div>
                <h3 className="font-headline text-lg font-bold text-zinc-900">Automation Pipeline Live</h3>
                <p className="text-sm text-zinc-500">{pipelineStatus.summary || 'Aguardando atualização...'}</p>
              </div>
              <button
                onClick={() => setShowPipelinePanel(false)}
                className="px-3 py-1.5 text-xs font-semibold rounded bg-zinc-100 hover:bg-zinc-200 text-zinc-700"
              >
                Fechar
              </button>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-[1fr_1.2fr] gap-0">
              <div className="p-6 border-r border-zinc-100 space-y-3">
                <p className="text-xs font-bold uppercase tracking-widest text-zinc-500">Etapas</p>
                {pipelineStatus.steps.length === 0 && (
                  <div className="text-sm text-zinc-500">Inicie o pipeline para ver o progresso.</div>
                )}
                {pipelineStatus.steps.map((step) => (
                  <div key={step.id} className="p-3 rounded-lg border border-zinc-200 bg-zinc-50">
                    <div className="flex items-center justify-between gap-3">
                      <p className="text-sm font-semibold text-zinc-900">{step.title}</p>
                      <span className={`text-[11px] font-bold uppercase tracking-wider ${step.status === 'done' ? 'text-green-600' : step.status === 'running' ? 'text-blue-600' : step.status === 'error' ? 'text-red-600' : 'text-zinc-500'}`}>
                        {step.status}
                      </span>
                    </div>
                    <p className="text-xs text-zinc-600 mt-1">{step.detail}</p>
                  </div>
                ))}
              </div>

              <div className="p-6">
                <p className="text-xs font-bold uppercase tracking-widest text-zinc-500 mb-3">Activity Log</p>
                <div ref={pipelineLogRef} className="h-72 overflow-y-auto rounded-lg border border-zinc-200 bg-zinc-950 text-zinc-100 p-3 font-mono text-xs leading-5 space-y-1">
                  {pipelineStatus.logs.length === 0 && <p className="text-zinc-400">Sem eventos ainda...</p>}
                  {pipelineStatus.logs.map((line, idx) => (
                    <p key={`${line}-${idx}`}>{line}</p>
                  ))}
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
