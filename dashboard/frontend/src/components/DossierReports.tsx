import { useEffect, useMemo, useState } from 'react';
import { FetchInterviews, GerarDossieEstudos } from "../../wailsjs/go/main/App";

interface EmailRecrutador {
  id: string;
  email: string;
  nome: string;
  classificacao: string;
  corpo: string;
}

export function DossierReports() {
  const [interviews, setInterviews] = useState<EmailRecrutador[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [dossierText, setDossierText] = useState('');
  const [loading, setLoading] = useState(true);
  const [isGenerating, setIsGenerating] = useState(false);

  useEffect(() => {
    const loadInterviews = async () => {
      setLoading(true);
      try {
        if ((window as any).go) {
          const items = await FetchInterviews();
          setInterviews(items || []);
          if (items && items.length > 0) {
            setSelectedId(items[0].id);
          }
        }
      } catch (err) {
        console.error('FetchInterviews failed:', err);
      } finally {
        setLoading(false);
      }
    };

    loadInterviews();
  }, []);

  const selectedInterview = useMemo(
    () => interviews.find((i) => i.id === selectedId),
    [interviews, selectedId],
  );

  const handleGenerate = async () => {
    if (!selectedInterview) {
      return;
    }

    setIsGenerating(true);
    try {
      const result = await GerarDossieEstudos(selectedInterview.corpo || '');
      setDossierText(result || 'Failed to generate dossier.');
    } catch (err) {
      setDossierText(`Error generating dossier: ${String(err)}`);
    } finally {
      setIsGenerating(false);
    }
  };

  const handleExport = () => {
    if (!dossierText.trim()) {
      return;
    }

    const blob = new Blob([dossierText], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = `dossier-${selectedId || 'report'}.txt`;
    anchor.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="w-full h-full flex flex-col overflow-hidden bg-white">
      {/* Cabeçalho contextual da tela de dossiês. */}
      <div className="flex items-center justify-between px-10 py-6 shrink-0 border-b border-zinc-100">
        <div className="flex items-center gap-2 text-on-surface-variant">
          <span className="text-[0.6875rem] font-medium uppercase tracking-widest opacity-60">Reports</span>
          <span className="material-symbols-outlined text-xs">chevron_right</span>
          <span className="text-[0.6875rem] font-semibold uppercase tracking-widest text-on-surface">Dossier Viewer</span>
        </div>
        <button onClick={handleExport} disabled={!dossierText.trim()} className="px-4 py-2 text-[0.875rem] font-medium rounded-lg flex items-center gap-2 border transition disabled:text-zinc-400 disabled:border-zinc-200 disabled:cursor-not-allowed text-zinc-800 border-zinc-300 hover:bg-zinc-50">
          <span className="material-symbols-outlined text-sm">picture_as_pdf</span>
          Export Dossier (.txt)
        </button>
      </div>

      <div className="flex-1 min-h-0 grid grid-cols-1 lg:grid-cols-[360px_1fr]">
        <aside className="border-r border-zinc-100 p-6 overflow-y-auto no-scrollbar">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xs font-bold tracking-widest uppercase text-zinc-500">Interview Invites</h2>
            <span className="text-xs font-mono text-zinc-400">{interviews.length}</span>
          </div>

          <div className="space-y-3">
            {loading && (
              <div className="p-4 text-sm text-zinc-500 border border-zinc-200 rounded-lg">Loading interviews...</div>
            )}

            {!loading && interviews.map((item) => (
              <button key={item.id} onClick={() => setSelectedId(item.id)} className={`w-full text-left p-4 rounded-lg border transition ${selectedId === item.id ? 'border-blue-400 bg-blue-50/50' : 'border-zinc-200 hover:border-zinc-300'}`}>
                <p className="text-sm font-semibold text-zinc-900 truncate">{item.nome || item.email}</p>
                <p className="text-xs text-zinc-500 truncate mt-1">{item.email}</p>
              </button>
            ))}

            {!loading && interviews.length === 0 && (
              <div className="p-4 text-sm text-zinc-500 border border-zinc-200 rounded-lg">No interview invites were found in the database.</div>
            )}
          </div>
        </aside>

        <section className="p-8 overflow-y-auto no-scrollbar flex flex-col gap-6">
          <div className="flex items-center justify-between gap-4">
            <div>
              <h3 className="text-lg font-semibold text-zinc-900">Strategic Dossier</h3>
              <p className="text-sm text-zinc-500">Select an interview and generate a plan with Gemini.</p>
            </div>
            <button onClick={handleGenerate} disabled={!selectedInterview || isGenerating} className="px-4 py-2 text-sm font-semibold rounded-lg bg-zinc-900 text-white hover:bg-zinc-800 disabled:bg-zinc-300 disabled:cursor-not-allowed">
              {isGenerating ? 'Generating...' : 'Generate Dossier'}
            </button>
          </div>

          {selectedInterview && (
            <div className="p-4 border border-zinc-200 rounded-lg bg-zinc-50">
              <p className="text-sm font-semibold text-zinc-900">{selectedInterview.nome || selectedInterview.email}</p>
              <p className="text-xs text-zinc-500 mt-1">{selectedInterview.email}</p>
              <p className="text-sm text-zinc-700 mt-3 whitespace-pre-wrap">{selectedInterview.corpo || 'No email body available.'}</p>
            </div>
          )}

          <div className="min-h-[320px] p-5 border border-zinc-200 rounded-lg bg-white text-sm text-zinc-700 whitespace-pre-wrap">
            {dossierText || 'Click Generate Dossier to process the selected email.'}
          </div>
        </section>
      </div>
    </div>
  );
}
