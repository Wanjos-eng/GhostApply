import { useState, useEffect } from 'react';
import { FetchEmails, FetchHistory, FetchInterviews, GerarDossieEstudos, RunPerformanceSuite } from "../../wailsjs/go/main/App";

interface EmailRecrutador {
  id: string;
  email: string;
  nome: string;
  classificacao: string;
  corpo: string;
}

interface VagaHistoricoDTO {
  vaga_id: string;
  titulo: string;
  empresa: string;
  url: string;
  vaga_status: string;
  candidatura_id: string;
  candidatura_status: string;
  recrutador_nome: string;
  recrutador_perfil: string;
  criado_em: string;
}

interface PerformanceSuiteDTO {
  ran_at: string;
  samples: number;
  database_ping_ms: number;
  database_ping_p95_ms: number;
  database_ping_p99_ms: number;
  fetch_history_ms: number;
  fetch_history_p95_ms: number;
  fetch_history_p99_ms: number;
  fetch_emails_ms: number;
  fetch_emails_p95_ms: number;
  fetch_emails_p99_ms: number;
  fetch_interviews_ms: number;
  fetch_interviews_p95_ms: number;
  fetch_interviews_p99_ms: number;
  history_rows: number;
  email_rows: number;
  interview_rows: number;
  total_suite_ms: number;
  total_suite_p95_ms: number;
  total_suite_p99_ms: number;
  database_reachable: boolean;
}

const formatPerf = (value?: number) => {
  if (value === undefined || value === null) {
    return '--';
  }
  return `${value.toFixed(3)}ms`;
};

export function DashboardView() {
  const [emails, setEmails] = useState<EmailRecrutador[]>([]);
  const [history, setHistory] = useState<VagaHistoricoDTO[]>([]);
  const [interviews, setInterviews] = useState<EmailRecrutador[]>([]);
  const [dossierText, setDossierText] = useState('');
  const [isGenerating, setIsGenerating] = useState(false);
  const [loading, setLoading] = useState(true);
  const [perfLoading, setPerfLoading] = useState(false);
  const [perf, setPerf] = useState<PerformanceSuiteDTO | null>(null);

  const runPerformance = async () => {
    setPerfLoading(true);
    try {
      const perfData = await RunPerformanceSuite();
      setPerf(perfData as PerformanceSuiteDTO);
    } catch (err) {
      console.error("Performance suite failed:", err);
    } finally {
      setPerfLoading(false);
    }
  };

  useEffect(() => {
    const fillData = async () => {
      setLoading(true);
      try {
        // Funciona apenas dentro do runtime do Wails.
        if ((window as any).go) {
          const [fetchedEmails, fetchedHistory, fetchedInterviews] = await Promise.all([
            FetchEmails(),
            FetchHistory(),
            FetchInterviews(),
          ]);

          setEmails(fetchedEmails || []);
          setHistory(fetchedHistory || []);
          setInterviews(fetchedInterviews || []);
        }
      } catch (err) {
        console.error("Dashboard data refresh failed:", err);
      } finally {
        setLoading(false);
      }
    };

    fillData();
    runPerformance();
    const dataIntervalId = setInterval(fillData, 10000);
    const perfIntervalId = setInterval(runPerformance, 60000);
    return () => {
      clearInterval(dataIntervalId);
      clearInterval(perfIntervalId);
    };
  }, []);

  const handleGerarDossie = async (body: string) => {
    setIsGenerating(true);
    setDossierText("Please wait... Gemini is generating the dossier outline.");
    try {
      const responseMarkdown = await GerarDossieEstudos(body);
      setDossierText(responseMarkdown);
    } catch (e) {
      setDossierText(`Error generating dossier: ${e}`);
    } finally {
      setIsGenerating(false);
    }
  };

  const closeDossier = () => setDossierText('');

  const nowDate = new Date().toISOString().slice(0, 10);
  const sentStatuses = new Set(["ENVIADA", "APLICADA", "CONFIRMADA"]);
  const detectedList = history.filter((v) => v.vaga_status !== "DESCARTADA" && v.vaga_status !== "REJEITADO_PRESENCIAL");
  const forgingList = history.filter((v) => v.candidatura_status === "FORJADO");
  const sentList = history.filter((v) => sentStatuses.has(v.candidatura_status));
  const sentToday = sentList.filter((v) => (v.criado_em || "").slice(0, 10) === nowDate).length;
  const metrics = {
    sentToday,
    pdfsForged: forgingList.length,
    detectedRemotes: detectedList.length,
  };

  return (
    <div className="w-full h-full p-8 space-y-8">
      
      {/* Cabeçalho principal e ação de performance */}
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-6">
        <div>
          <h1 className="font-headline font-bold text-[3.5rem] leading-tight text-on-surface tracking-tight">GhostApply Dashboard</h1>
          <p className="text-on-surface-variant max-w-xl mt-2">Precision orchestration of automated remote career deployment. Forging documents and detecting opportunities in real-time.</p>
        </div>
        <div className="flex p-1 bg-surface-container rounded-lg shrink-0">
          <button
            onClick={runPerformance}
            className="px-4 py-1.5 text-xs font-semibold rounded bg-white shadow-sm text-on-surface transition-all"
          >
            {perfLoading ? 'Running Perf...' : 'Run Performance'}
          </button>
        </div>
      </div>

      {/* Linha de métricas */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="bg-surface-container-lowest p-6 rounded shadow-[0px_4px_20px_rgba(0,0,0,0.04)]">
          <p className="text-on-surface-variant text-[0.6875rem] font-medium uppercase tracking-wider mb-1">Sent Today</p>
          <div className="flex items-baseline gap-2">
            <span className="font-mono text-4xl font-bold text-on-surface">{metrics.sentToday}</span>
            <span className="text-zinc-500 text-xs font-mono font-bold">UPDATED 10S</span>
          </div>
        </div>
        <div className="bg-surface-container-lowest p-6 rounded shadow-[0px_4px_20px_rgba(0,0,0,0.04)]">
          <p className="text-on-surface-variant text-[0.6875rem] font-medium uppercase tracking-wider mb-1">PDFs Forged</p>
          <div className="flex items-baseline gap-2">
            <span className="font-mono text-4xl font-bold text-on-surface">{metrics.pdfsForged}</span>
            <span className="text-zinc-400 text-xs font-mono font-bold">FORGED</span>
          </div>
        </div>
        <div className="bg-surface-container-lowest p-6 rounded shadow-[0px_4px_20px_rgba(0,0,0,0.04)]">
          <p className="text-on-surface-variant text-[0.6875rem] font-medium uppercase tracking-wider mb-1">Detected Remotes</p>
          <div className="flex items-baseline gap-2">
            <span className="font-mono text-4xl font-bold text-on-surface">{metrics.detectedRemotes}</span>
            <span className="text-zinc-400 text-xs font-mono font-bold">VALID JOBS</span>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <div className="bg-surface-container-lowest p-5 rounded shadow-[0px_4px_20px_rgba(0,0,0,0.04)]">
          <p className="text-on-surface-variant text-[0.6875rem] font-medium uppercase tracking-wider mb-1">DB Ping</p>
          <span className="font-mono text-3xl font-bold text-on-surface">{formatPerf(perf?.database_ping_ms)}</span>
          <p className="text-[0.65rem] text-zinc-500 mt-2">P95 {formatPerf(perf?.database_ping_p95_ms)} | P99 {formatPerf(perf?.database_ping_p99_ms)}</p>
        </div>
        <div className="bg-surface-container-lowest p-5 rounded shadow-[0px_4px_20px_rgba(0,0,0,0.04)]">
          <p className="text-on-surface-variant text-[0.6875rem] font-medium uppercase tracking-wider mb-1">Fetch History</p>
          <span className="font-mono text-3xl font-bold text-on-surface">{formatPerf(perf?.fetch_history_ms)}</span>
          <p className="text-[0.65rem] text-zinc-500 mt-2">P95 {formatPerf(perf?.fetch_history_p95_ms)} | P99 {formatPerf(perf?.fetch_history_p99_ms)}</p>
        </div>
        <div className="bg-surface-container-lowest p-5 rounded shadow-[0px_4px_20px_rgba(0,0,0,0.04)]">
          <p className="text-on-surface-variant text-[0.6875rem] font-medium uppercase tracking-wider mb-1">Fetch Emails</p>
          <span className="font-mono text-3xl font-bold text-on-surface">{formatPerf(perf?.fetch_emails_ms)}</span>
          <p className="text-[0.65rem] text-zinc-500 mt-2">P95 {formatPerf(perf?.fetch_emails_p95_ms)} | P99 {formatPerf(perf?.fetch_emails_p99_ms)}</p>
        </div>
        <div className="bg-surface-container-lowest p-5 rounded shadow-[0px_4px_20px_rgba(0,0,0,0.04)]">
          <p className="text-on-surface-variant text-[0.6875rem] font-medium uppercase tracking-wider mb-1">Suite Total</p>
          <span className="font-mono text-3xl font-bold text-on-surface">{formatPerf(perf?.total_suite_ms)}</span>
          <p className="text-[0.65rem] text-zinc-500 mt-2">P95 {formatPerf(perf?.total_suite_p95_ms)} | P99 {formatPerf(perf?.total_suite_p99_ms)}</p>
        </div>
      </div>

      <div className="text-xs text-zinc-500 font-mono">
        Perf sample size: {perf ? perf.samples : '--'} runs | Last run: {perf ? perf.ran_at : '--'}
      </div>

      {/* Board operacional */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6 min-h-[600px]">
        {/* Coluna: detectadas */}
        <div className="space-y-4">
          <div className="flex items-center justify-between px-2">
            <h3 className="text-[0.6875rem] font-bold uppercase tracking-widest text-on-surface">Detected</h3>
            <span className="font-mono text-[0.6875rem] text-zinc-400">{detectedList.length}</span>
          </div>
          <div className="space-y-3">
            {detectedList.slice(0, 8).map((v) => (
              <a key={`${v.vaga_id}-detected`} href={v.url} target="_blank" rel="noreferrer" className="block bg-white border border-zinc-200 p-4 rounded shadow-[0px_10px_40px_rgba(0,0,0,0.02)] hover:border-zinc-400 transition">
                <h4 className="font-headline font-semibold text-sm mb-1 line-clamp-2">{v.titulo || "Job"}</h4>
                <p className="text-on-surface-variant text-xs truncate">{v.empresa || "Company"}</p>
              </a>
            ))}
            {detectedList.length === 0 && !loading && (
              <div className="p-4 text-center text-xs text-zinc-400 border border-dashed border-zinc-200 rounded-lg">
                No jobs detected at the moment.
              </div>
            )}
          </div>
        </div>

        {/* Coluna: em forja */}
        <div className="space-y-4">
          <div className="flex items-center justify-between px-2">
            <h3 className="text-[0.6875rem] font-bold uppercase tracking-widest text-on-surface">Forging</h3>
            <span className="font-mono text-[0.6875rem] text-zinc-400">{forgingList.length}</span>
          </div>
          <div className="space-y-3">
            {forgingList.slice(0, 8).map((v) => (
              <a key={`${v.vaga_id}-forging`} href={v.url} target="_blank" rel="noreferrer" className="block bg-white border border-amber-200 p-4 rounded shadow-[0px_10px_40px_rgba(0,0,0,0.02)] hover:border-amber-400 transition">
                <h4 className="font-headline font-semibold text-sm mb-1 line-clamp-2">{v.titulo || "Job"}</h4>
                <p className="text-on-surface-variant text-xs truncate">{v.empresa || "Company"}</p>
              </a>
            ))}
            {forgingList.length === 0 && !loading && (
              <div className="p-4 text-center text-xs text-zinc-400 border border-dashed border-zinc-200 rounded-lg">
                No applications are being forged right now.
              </div>
            )}
          </div>
        </div>

        {/* Coluna: enviadas */}
        <div className="space-y-4">
          <div className="flex items-center justify-between px-2">
            <h3 className="text-[0.6875rem] font-bold uppercase tracking-widest text-on-surface">Sent</h3>
            <span className="font-mono text-[0.6875rem] text-zinc-400">{sentList.length}</span>
          </div>
          <div className="space-y-3">
            {sentList.slice(0, 8).map((v) => (
              <a key={`${v.vaga_id}-sent`} href={v.url} target="_blank" rel="noreferrer" className="block bg-white border border-emerald-200 p-4 rounded shadow-[0px_10px_40px_rgba(0,0,0,0.02)] hover:border-emerald-400 transition">
                <h4 className="font-headline font-semibold text-sm mb-1 line-clamp-2">{v.titulo || "Job"}</h4>
                <p className="text-on-surface-variant text-xs truncate">{v.empresa || "Company"}</p>
              </a>
            ))}
            {sentList.length === 0 && !loading && (
              <div className="p-4 text-center text-xs text-zinc-400 border border-dashed border-zinc-200 rounded-lg">
                No applications have been sent yet.
              </div>
            )}
          </div>
        </div>

        {/* Coluna: entrevistas (dados reais do banco) */}
        <div className="space-y-4">
          <div className="flex items-center justify-between px-2">
            <h3 className="text-[0.6875rem] font-bold uppercase tracking-widest text-on-surface">Interviews</h3>
            <span className="font-mono text-[0.6875rem] text-zinc-400">{interviews.length}</span>
          </div>
          <div className="space-y-3">
            
            {/* Dados reais vindos do banco */}
            {interviews.map((e) => (
               <div key={e.id} className="bg-white border border-blue-200 p-4 rounded shadow-[0px_10px_40px_rgba(0,0,0,0.02)]">
                <div className="flex flex-wrap items-center gap-2 mb-2 justify-between">
                  <span className="px-2 py-0.5 bg-blue-50 text-blue-700 text-[9px] font-bold rounded-full uppercase tracking-tighter">High Signal</span>
                  <button onClick={() => handleGerarDossie(e.corpo)} className="px-2 py-0.5 bg-zinc-900 text-white text-[9px] font-bold rounded hover:bg-zinc-700 cursor-pointer transition">Generate Dossier (Gemini)</button>
                </div>
                <h4 className="font-headline font-semibold text-sm mb-1">{e.nome || e.email}</h4>
                <p className="text-on-surface-variant text-xs mb-3 truncate">{e.corpo}</p>
                <div className="flex items-center gap-2 mt-4 pt-4 border-t border-zinc-50">
                  <span className="material-symbols-outlined text-[1rem] text-zinc-400">calendar_today</span>
                  <span className="font-mono text-[10px] text-zinc-500">Scheduled via IMAP</span>
                </div>
              </div>
            ))}

            {/* Fallback visual quando não há entrevistas */}
            {interviews.length === 0 && !loading && (
              <div className="p-4 text-center text-xs text-zinc-400 border border-dashed border-zinc-200 rounded-lg">
                 No interview invites found right now.
              </div>
            )}

          </div>
        </div>
      </div>
      
      {/* Modal de dossiê */}
      {dossierText && (
        <div className="fixed inset-0 bg-zinc-900/40 backdrop-blur-sm flex items-center justify-center z-[1000] p-4">
          <div className="bg-white w-full max-w-4xl max-h-[85vh] rounded-xl flex flex-col shadow-2xl">
            <h2 className="m-0 p-6 border-b border-zinc-100 text-zinc-900 bg-zinc-50 rounded-t-xl font-bold font-headline">Strategic Dossier (Gemini)</h2>
            <div className="p-6 overflow-y-auto text-zinc-700 whitespace-pre-wrap font-mono text-sm leading-relaxed">
              {dossierText}
            </div>
            <button 
                className="m-6 p-3 bg-zinc-900 text-white rounded-lg font-semibold cursor-pointer self-end hover:bg-zinc-800" 
                onClick={closeDossier} 
                disabled={isGenerating}>
              Close Target Map
            </button>
          </div>
        </div>
      )}

    </div>
  );
}
