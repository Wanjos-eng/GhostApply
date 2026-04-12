import { useState, useEffect } from 'react';
import { FetchEmails, FetchHistory, FetchInterviews, GerarDossieEstudos } from "../../wailsjs/go/main/App";

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

export function DashboardView() {
  const [emails, setEmails] = useState<EmailRecrutador[]>([]);
  const [history, setHistory] = useState<VagaHistoricoDTO[]>([]);
  const [interviews, setInterviews] = useState<EmailRecrutador[]>([]);
  const [dossierText, setDossierText] = useState('');
  const [isGenerating, setIsGenerating] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fillData = async () => {
      setLoading(true);
      try {
        // Only works inside Wails runtime
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
    const intervalId = setInterval(fillData, 10000);
    return () => clearInterval(intervalId);
  }, []);

  const handleGerarDossie = async (body: string) => {
    setIsGenerating(true);
    setDossierText("Aguarde... o LLM Gemini está sendo acionado e gerando seu cronograma base.");
    try {
      const responseMarkdown = await GerarDossieEstudos(body);
      setDossierText(responseMarkdown);
    } catch (e) {
      setDossierText(`Erro ao gerar dossiê: ${e}`);
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
    <div className="p-8 space-y-8 overflow-y-auto">
      
      {/* Editorial Header & Filter */}
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-6">
        <div>
          <h1 className="font-headline font-bold text-[3.5rem] leading-tight text-on-surface tracking-tight">GhostApply Dashboard</h1>
          <p className="text-on-surface-variant max-w-xl mt-2">Precision orchestration of automated remote career deployment. Forging documents and detecting opportunities in real-time.</p>
        </div>
        <div className="flex p-1 bg-surface-container rounded-lg shrink-0">
          <button className="px-4 py-1.5 text-xs font-semibold rounded bg-white shadow-sm text-on-surface transition-all">Today</button>
          <button className="px-4 py-1.5 text-xs font-medium text-on-surface-variant hover:text-on-surface transition-all">Week</button>
          <button className="px-4 py-1.5 text-xs font-medium text-on-surface-variant hover:text-on-surface transition-all">All-Time</button>
        </div>
      </div>

      {/* Metrics Row */}
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
            <span className="text-zinc-400 text-xs font-mono font-bold">FORJADO</span>
          </div>
        </div>
        <div className="bg-surface-container-lowest p-6 rounded shadow-[0px_4px_20px_rgba(0,0,0,0.04)]">
          <p className="text-on-surface-variant text-[0.6875rem] font-medium uppercase tracking-wider mb-1">Detected Remotes</p>
          <div className="flex items-baseline gap-2">
            <span className="font-mono text-4xl font-bold text-on-surface">{metrics.detectedRemotes}</span>
            <span className="text-zinc-400 text-xs font-mono font-bold">VALID VAGAS</span>
          </div>
        </div>
      </div>

      {/* Kanban Board */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6 min-h-[600px]">
        {/* Column: Detected */}
        <div className="space-y-4">
          <div className="flex items-center justify-between px-2">
            <h3 className="text-[0.6875rem] font-bold uppercase tracking-widest text-on-surface">Detected</h3>
            <span className="font-mono text-[0.6875rem] text-zinc-400">{detectedList.length}</span>
          </div>
          <div className="space-y-3">
            {detectedList.slice(0, 8).map((v) => (
              <a key={`${v.vaga_id}-detected`} href={v.url} target="_blank" rel="noreferrer" className="block bg-white border border-zinc-200 p-4 rounded shadow-[0px_10px_40px_rgba(0,0,0,0.02)] hover:border-zinc-400 transition">
                <h4 className="font-headline font-semibold text-sm mb-1 line-clamp-2">{v.titulo || "Vaga"}</h4>
                <p className="text-on-surface-variant text-xs truncate">{v.empresa || "Empresa"}</p>
              </a>
            ))}
            {detectedList.length === 0 && !loading && (
              <div className="p-4 text-center text-xs text-zinc-400 border border-dashed border-zinc-200 rounded-lg">
                Sem vagas detectadas neste momento.
              </div>
            )}
          </div>
        </div>

        {/* Column: Forging */}
        <div className="space-y-4">
          <div className="flex items-center justify-between px-2">
            <h3 className="text-[0.6875rem] font-bold uppercase tracking-widest text-on-surface">Forging</h3>
            <span className="font-mono text-[0.6875rem] text-zinc-400">{forgingList.length}</span>
          </div>
          <div className="space-y-3">
            {forgingList.slice(0, 8).map((v) => (
              <a key={`${v.vaga_id}-forging`} href={v.url} target="_blank" rel="noreferrer" className="block bg-white border border-amber-200 p-4 rounded shadow-[0px_10px_40px_rgba(0,0,0,0.02)] hover:border-amber-400 transition">
                <h4 className="font-headline font-semibold text-sm mb-1 line-clamp-2">{v.titulo || "Vaga"}</h4>
                <p className="text-on-surface-variant text-xs truncate">{v.empresa || "Empresa"}</p>
              </a>
            ))}
            {forgingList.length === 0 && !loading && (
              <div className="p-4 text-center text-xs text-zinc-400 border border-dashed border-zinc-200 rounded-lg">
                Nenhuma candidatura em forja agora.
              </div>
            )}
          </div>
        </div>

        {/* Column: Sent */}
        <div className="space-y-4">
          <div className="flex items-center justify-between px-2">
            <h3 className="text-[0.6875rem] font-bold uppercase tracking-widest text-on-surface">Sent</h3>
            <span className="font-mono text-[0.6875rem] text-zinc-400">{sentList.length}</span>
          </div>
          <div className="space-y-3">
            {sentList.slice(0, 8).map((v) => (
              <a key={`${v.vaga_id}-sent`} href={v.url} target="_blank" rel="noreferrer" className="block bg-white border border-emerald-200 p-4 rounded shadow-[0px_10px_40px_rgba(0,0,0,0.02)] hover:border-emerald-400 transition">
                <h4 className="font-headline font-semibold text-sm mb-1 line-clamp-2">{v.titulo || "Vaga"}</h4>
                <p className="text-on-surface-variant text-xs truncate">{v.empresa || "Empresa"}</p>
              </a>
            ))}
            {sentList.length === 0 && !loading && (
              <div className="p-4 text-center text-xs text-zinc-400 border border-dashed border-zinc-200 rounded-lg">
                Nenhuma candidatura enviada ainda.
              </div>
            )}
          </div>
        </div>

        {/* Column: Interviews (Dynamic DB data mixed with their layout) */}
        <div className="space-y-4">
          <div className="flex items-center justify-between px-2">
            <h3 className="text-[0.6875rem] font-bold uppercase tracking-widest text-on-surface">Interviews</h3>
            <span className="font-mono text-[0.6875rem] text-zinc-400">{interviews.length}</span>
          </div>
          <div className="space-y-3">
            
            {/* Dynamic Real Data */}
            {interviews.map((e) => (
               <div key={e.id} className="bg-white border border-blue-200 p-4 rounded shadow-[0px_10px_40px_rgba(0,0,0,0.02)]">
                <div className="flex flex-wrap items-center gap-2 mb-2 justify-between">
                  <span className="px-2 py-0.5 bg-blue-50 text-blue-700 text-[9px] font-bold rounded-full uppercase tracking-tighter">High Signal</span>
                  <button onClick={() => handleGerarDossie(e.corpo)} className="px-2 py-0.5 bg-zinc-900 text-white text-[9px] font-bold rounded hover:bg-zinc-700 cursor-pointer transition">Gerar Dossiê (Gemini)</button>
                </div>
                <h4 className="font-headline font-semibold text-sm mb-1">{e.nome || e.email}</h4>
                <p className="text-on-surface-variant text-xs mb-3 truncate">{e.corpo}</p>
                <div className="flex items-center gap-2 mt-4 pt-4 border-t border-zinc-50">
                  <span className="material-symbols-outlined text-[1rem] text-zinc-400">calendar_today</span>
                  <span className="font-mono text-[10px] text-zinc-500">Scheduled via IMAP</span>
                </div>
              </div>
            ))}

            {/* Default Static Template layout from user if DB has no hits yet */}
            {interviews.length === 0 && !loading && (
              <div className="p-4 text-center text-xs text-zinc-400 border border-dashed border-zinc-200 rounded-lg">
                 Nenhum convite de entrevista encontrado no momento.
              </div>
            )}

          </div>
        </div>
      </div>
      
      {/* Dossier Modal Overlay */}
      {dossierText && (
        <div className="fixed inset-0 bg-zinc-900/40 backdrop-blur-sm flex items-center justify-center z-[1000] p-4">
          <div className="bg-white w-full max-w-4xl max-h-[85vh] rounded-xl flex flex-col shadow-2xl">
            <h2 className="m-0 p-6 border-b border-zinc-100 text-zinc-900 bg-zinc-50 rounded-t-xl font-bold font-headline">Dossiê Estratégico (Gemini)</h2>
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
