import { useState, useEffect } from 'react';
import { FetchEmails, GerarDossieEstudos } from "../../wailsjs/go/main/App";

interface EmailRecrutador {
  id: string;
  email: string;
  nome: string;
  classificacao: string;
  corpo: string;
}

export function DashboardView() {
  const [emails, setEmails] = useState<EmailRecrutador[]>([]);
  const [dossierText, setDossierText] = useState('');
  const [isGenerating, setIsGenerating] = useState(false);
  
  // Dashboard Metrics State (ready to receive increments from background jobs)
  const [metrics, setMetrics] = useState({
    sentToday: 0,
    pdfsForged: 0,
    detectedRemotes: 0
  });

  useEffect(() => {
    const fillEmails = async () => {
      try {
        // Only works inside Wails runtime
        if ((window as any).go) {
           const fetched = await FetchEmails();
           if (fetched) setEmails(fetched);
        }
      } catch (err) {
        console.error("FetchEmails failed:", err);
      }
    };
    
    fillEmails();
    const intervalId = setInterval(fillEmails, 5000);
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

  const entrevistas = emails.filter(e => e.classificacao === "ENTREVISTA");

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
            <span className="text-zinc-500 text-xs font-mono font-bold">AWAITING ENGINE</span>
          </div>
        </div>
        <div className="bg-surface-container-lowest p-6 rounded shadow-[0px_4px_20px_rgba(0,0,0,0.04)]">
          <p className="text-on-surface-variant text-[0.6875rem] font-medium uppercase tracking-wider mb-1">PDFs Forged</p>
          <div className="flex items-baseline gap-2">
            <span className="font-mono text-4xl font-bold text-on-surface">{metrics.pdfsForged}</span>
            <span className="text-zinc-400 text-xs font-mono font-bold">NONE</span>
          </div>
        </div>
        <div className="bg-surface-container-lowest p-6 rounded shadow-[0px_4px_20px_rgba(0,0,0,0.04)]">
          <p className="text-on-surface-variant text-[0.6875rem] font-medium uppercase tracking-wider mb-1">Detected Remotes</p>
          <div className="flex items-baseline gap-2">
            <span className="font-mono text-4xl font-bold text-on-surface">{metrics.detectedRemotes}</span>
            <span className="text-zinc-400 text-xs font-mono font-bold">PENDING SCAN</span>
          </div>
        </div>
      </div>

      {/* Kanban Board */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6 min-h-[600px]">
        {/* Column: Detected */}
        <div className="space-y-4">
          <div className="flex items-center justify-between px-2">
            <h3 className="text-[0.6875rem] font-bold uppercase tracking-widest text-on-surface">Detected</h3>
            <span className="font-mono text-[0.6875rem] text-zinc-400">0</span>
          </div>
          <div className="space-y-3">
             <div className="p-4 text-center text-xs text-zinc-400 border border-dashed border-zinc-200 rounded-lg">
                Nenhuma vaga identificada. Agendando scan...
             </div>
          </div>
        </div>

        {/* Column: Forging */}
        <div className="space-y-4">
          <div className="flex items-center justify-between px-2">
            <h3 className="text-[0.6875rem] font-bold uppercase tracking-widest text-on-surface">Forging</h3>
            <span className="font-mono text-[0.6875rem] text-zinc-400">0</span>
          </div>
          <div className="space-y-3">
             <div className="p-4 text-center text-xs text-zinc-400 border border-dashed border-zinc-200 rounded-lg">
                Fila de forja vazia. Aguardando oportunidades...
             </div>
          </div>
        </div>

        {/* Column: Sent */}
        <div className="space-y-4">
          <div className="flex items-center justify-between px-2">
            <h3 className="text-[0.6875rem] font-bold uppercase tracking-widest text-on-surface">Sent</h3>
            <span className="font-mono text-[0.6875rem] text-zinc-400">0</span>
          </div>
          <div className="space-y-3">
             <div className="p-4 text-center text-xs text-zinc-400 border border-dashed border-zinc-200 rounded-lg">
                Aguardando disparo de documentação...
             </div>
          </div>
        </div>

        {/* Column: Interviews (Dynamic DB data mixed with their layout) */}
        <div className="space-y-4">
          <div className="flex items-center justify-between px-2">
            <h3 className="text-[0.6875rem] font-bold uppercase tracking-widest text-on-surface">Interviews</h3>
            <span className="font-mono text-[0.6875rem] text-zinc-400">{entrevistas.length > 0 ? entrevistas.length : '0'}</span>
          </div>
          <div className="space-y-3">
            
            {/* Dynamic Real Data */}
            {entrevistas.map((e, index) => (
               <div key={index} className="bg-white border border-blue-200 p-4 rounded shadow-[0px_10px_40px_rgba(0,0,0,0.02)]">
                <div className="flex flex-wrap items-center gap-2 mb-2 justify-between">
                  <span className="px-2 py-0.5 bg-blue-50 text-blue-700 text-[9px] font-bold rounded-full uppercase tracking-tighter">High Signal</span>
                  <button onClick={() => handleGerarDossie(e.corpo)} className="px-2 py-0.5 bg-zinc-900 text-white text-[9px] font-bold rounded hover:bg-zinc-700 cursor-pointer transition">Gerar Dossiê (Gemini)</button>
                </div>
                <h4 className="font-headline font-semibold text-sm mb-1">{e.email}</h4>
                <p className="text-on-surface-variant text-xs mb-3 truncate">{e.corpo}</p>
                <div className="flex items-center gap-2 mt-4 pt-4 border-t border-zinc-50">
                  <span className="material-symbols-outlined text-[1rem] text-zinc-400">calendar_today</span>
                  <span className="font-mono text-[10px] text-zinc-500">Scheduled via IMAP</span>
                </div>
              </div>
            ))}

            {/* Default Static Template layout from user if DB has no hits yet */}
            {entrevistas.length === 0 && (
              <div className="p-4 text-center text-xs text-zinc-400 border border-dashed border-zinc-200 rounded-lg">
                 O IMAP não detectou convites na sua caixa de entrada.
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
