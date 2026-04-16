import { useState, useEffect } from 'react';

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

export const HistoryView = () => {
  const [history, setHistory] = useState<VagaHistoricoDTO[]>([]);
  const [filter, setFilter] = useState<'ALL' | 'MANUAL' | 'APLICADAS' | 'REJEITADAS'>('ALL');
  const [loading, setLoading] = useState(true);
  
  const [outreachModal, setOutreachModal] = useState<{open: boolean, loading: boolean, text: string, recruiter: string}>({
    open: false, loading: false, text: '', recruiter: ''
  });

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    setLoading(true);
    try {
      const app = (window as any).go?.main?.App;
      if (!app?.FetchHistory) {
        setHistory([]);
        return;
      }
      const data: VagaHistoricoDTO[] = await app.FetchHistory();
      setHistory(data || []);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  const generateOutreach = async (recruiterName: string, roleName: string) => {
    setOutreachModal({ open: true, loading: true, text: '', recruiter: recruiterName || 'Recrutador' });
    try {
      const app = (window as any).go?.main?.App;
      if (!app?.GenerateOutreachMessage) {
        setOutreachModal(prev => ({ ...prev, loading: false, text: 'Runtime Wails indisponível para gerar outreach.' }));
        return;
      }
      const msg = await app.GenerateOutreachMessage(recruiterName, roleName);
      setOutreachModal(prev => ({ ...prev, loading: false, text: msg }));
    } catch (e) {
      setOutreachModal(prev => ({ ...prev, loading: false, text: 'Error generating message: ' + String(e) }));
    }
  };

  // Estado derivado para aplicar os filtros sem duplicar a origem dos dados.
  const filteredData = history.filter(v => {
    if (filter === 'ALL') return true;
    if (filter === 'MANUAL') return v.vaga_status === 'ALERTA_MANUAL';
    if (filter === 'APLICADAS') return v.candidatura_status === 'APLICADA' || v.candidatura_status === 'ENVIADA';
    if (filter === 'REJEITADAS') return v.vaga_status === 'REJEITADO_PRESENCIAL' || v.vaga_status === 'DESCARTADA' || v.candidatura_status === 'REJEITADA';
    return true;
  });

  const getStatusBadge = (vagaStatus: string, candStatus: string) => {
    // Alertas manuais ficam em destaque porque exigem revisão imediata.
    if (vagaStatus === 'ALERTA_MANUAL') {
      return <span className="px-3 py-1 bg-rose-500/10 text-rose-600 border border-rose-500/20 rounded-full text-xs font-bold uppercase tracking-wider animate-pulse flex items-center gap-1.5"><span className="w-1.5 h-1.5 bg-rose-500 rounded-full"></span>Manual Alert</span>;
    }
    // Itens descartados ou rejeitados entram na mesma categoria visual.
    if (vagaStatus === 'REJEITADO_PRESENCIAL' || vagaStatus === 'DESCARTADA') {
      return <span className="px-3 py-1 bg-gray-100 text-gray-500 border border-gray-200 rounded-full text-xs font-bold uppercase tracking-wider">Discarded</span>;
    }
    if (candStatus === 'ERRO') {
      return <span className="px-3 py-1 bg-orange-100 text-orange-600 border border-orange-200 rounded-full text-xs font-bold uppercase tracking-wider">Failed</span>;
    }
    // Estados de sucesso reaproveitam o mesmo badge para manter consistência.
    if (candStatus === 'APLICADA' || candStatus === 'ENVIADA' || candStatus === 'CONFIRMADA') {
      return <span className="px-3 py-1 bg-teal-500/10 text-teal-600 border border-teal-500/20 rounded-full text-xs font-bold uppercase tracking-wider flex items-center gap-1.5"><span className="w-1.5 h-1.5 bg-teal-500 rounded-full"></span>Applied</span>;
    }
    // O restante segue como pendente até o backend trazer um estado mais específico.
    return <span className="px-3 py-1 bg-indigo-50 text-indigo-600 border border-indigo-100 rounded-full text-xs font-bold uppercase tracking-wider">Pending</span>;
  };

  return (
    <div className="w-full h-full p-10 bg-gradient-to-br from-[#f9f9f9] to-gray-100/50">
      
      {/* Cabeçalho e filtros */}
      <div className="flex justify-between items-end mb-8">
        <div>
          <h1 className="text-3xl font-extrabold text-[#1a1c1c] tracking-tight mb-2">Operational Terminal</h1>
          <p className="text-gray-500 text-sm font-medium">Automatic engagement control and manual alert tracking.</p>
        </div>
        <div className="flex gap-2">
          {['ALL', 'MANUAL', 'APLICADAS', 'REJEITADAS'].map(f => (
            <button
              key={f}
              onClick={() => setFilter(f as any)}
              className={`px-4 py-2 text-xs font-bold uppercase tracking-wider rounded-xl transition-all duration-300 ${
                filter === f
                  ? f === 'MANUAL' 
                    ? 'bg-rose-500 text-white shadow-md shadow-rose-500/20'
                    : 'bg-[#1a1c1c] text-white shadow-md shadow-black/10'
                  : 'bg-white text-gray-400 border border-gray-100 hover:bg-gray-50 hover:text-gray-700'
              }`}
            >
              {f === 'ALL' ? 'All' : f === 'MANUAL' ? 'Manual Alerts' : f === 'APLICADAS' ? 'Applied' : 'Rejected'}
            </button>
          ))}
        </div>
      </div>

      {loading ? (
        <div className="flex justify-center items-center h-64">
           <div className="w-8 h-8 border-4 border-indigo-500 border-t-transparent rounded-full animate-spin"></div>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {filteredData.length === 0 ? (
            <div className="col-span-full py-20 text-center text-gray-400 font-medium">No events recorded in this tab.</div>
          ) : (
            filteredData.map(v => (
              <div key={v.vaga_id} className="relative group bg-white/70 backdrop-blur-md border border-white/40 shadow-sm hover:shadow-lg rounded-2xl p-6 transition-all duration-300 flex flex-col justify-between overflow-hidden">
                
                {/* Destaque de fundo para alertas manuais. */}
                {v.vaga_status === 'ALERTA_MANUAL' && (
                  <div className="absolute top-0 right-0 w-32 h-32 bg-rose-500/5 rounded-full blur-3xl -mr-10 -mt-10 pointer-events-none"></div>
                )}
                
                <div>
                  <div className="flex justify-between items-start mb-4">
                    {getStatusBadge(v.vaga_status, v.candidatura_status)}
                    {v.url ? (
                      <a href={v.url} target="_blank" rel="noreferrer" className="text-gray-400 hover:text-indigo-600 transition-colors" title="Open job link">
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg>
                      </a>
                    ) : null}
                  </div>

                  <h3 className="font-bold text-[#1a1c1c] text-lg leading-snug mb-1 line-clamp-2">{v.titulo || 'Unknown Job'}</h3>
                  <p className="text-gray-500 text-sm font-medium mb-4">{v.empresa || 'Confidential Company'}</p>
                </div>
                
                <div className="mt-4 pt-4 border-t border-gray-100 flex justify-between items-center">
                  <div className="text-xs text-gray-400 font-medium">
                    {new Date(v.criado_em).toLocaleDateString('en-US', {day: '2-digit', month: 'short', year: 'numeric'})}
                  </div>

                  <div className="flex items-center gap-2">
                    {v.url ? (
                      <a
                        href={v.url}
                        target="_blank"
                        rel="noreferrer"
                        className="px-3 py-1.5 border border-indigo-100 text-indigo-600 hover:bg-indigo-50 rounded-lg text-[11px] font-bold uppercase tracking-wider transition-all"
                      >
                        Open Link
                      </a>
                    ) : (
                      <span className="px-3 py-1.5 border border-gray-200 text-gray-400 rounded-lg text-[11px] font-bold uppercase tracking-wider">
                        No Link
                      </span>
                    )}

                    {/* Mostra a ação de outreach só para alertas manuais. */}
                    {v.vaga_status === 'ALERTA_MANUAL' && (
                      <button
                        onClick={() => generateOutreach(v.recrutador_nome, v.titulo)}
                        className="px-4 py-2 bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-500 hover:to-indigo-400 text-white rounded-lg text-xs font-bold shadow-md shadow-indigo-500/20 transition-all flex items-center gap-2 group-hover:-translate-y-0.5"
                      >
                        <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" /></svg>
                        Outreach Message
                      </button>
                    )}
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      )}

      {/* Modal de outreach */}
      {outreachModal.open && (
         <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/40 backdrop-blur-sm transition-all duration-300">
           <div className="bg-white rounded-2xl shadow-xl border border-gray-100 w-full max-w-2xl overflow-hidden animate-in fade-in zoom-in-95 duration-200">
              
              <div className="px-6 py-4 border-b border-gray-100 bg-gray-50 flex justify-between items-center">
                  <h2 className="font-extrabold text-[#1a1c1c] flex items-center gap-2">
                  <span className="material-symbols-outlined text-indigo-600">offline_bolt</span>
                  Outreach Synthesizer
                </h2>
                <button onClick={() => setOutreachModal({...outreachModal, open: false})} className="text-gray-400 hover:text-gray-700">
                  <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" /></svg>
                </button>
              </div>

              <div className="p-8">
                 {outreachModal.loading ? (
                    <div className="flex flex-col items-center justify-center space-y-4 py-10">
                      <div className="w-10 h-10 border-4 border-indigo-200 border-t-indigo-600 rounded-full animate-spin"></div>
                      <p className="text-gray-500 font-medium text-sm">Crafting ATS-safe outreach for {outreachModal.recruiter}...</p>
                    </div>
                 ) : (
                    <div className="space-y-6">
                      <div className="bg-[#1a1c1c] text-green-400 p-6 rounded-xl font-mono text-sm leading-relaxed whitespace-pre-wrap tracking-tight shadow-inner relative group">
                        
                        <div className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity">
                          <button 
                            onClick={() => {
                              navigator.clipboard.writeText(outreachModal.text);
                              alert('Copied to clipboard!');
                            }}
                            className="bg-white/10 hover:bg-white/20 px-3 py-1.5 rounded-lg text-white font-sans text-xs font-bold transition-all"
                          >
                            Copy Text
                          </button>
                        </div>
                        
                        {outreachModal.text}
                      </div>
                      
                      <div className="px-4 py-3 bg-amber-50 text-amber-800 border border-amber-200 rounded-xl text-xs font-medium flex items-start gap-3">
                        <span className="material-symbols-outlined text-amber-500">lightbulb</span>
                        <p><strong>Tactical Note:</strong> LinkedIn invites without Sales Navigator have a 300-character limit. If the generated text is too long, trim unnecessary greetings and go straight to the technical pain point you share with the recruiter.</p>
                      </div>
                    </div>
                 )}
              </div>

           </div>
         </div>
      )}

    </div>
  );
};
