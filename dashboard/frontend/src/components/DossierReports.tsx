export function DossierReports() {
  return (
    <div className="flex-1 flex flex-col h-full overflow-hidden bg-white w-full">
      {/* Contextual Header */}
      <div className="flex items-center justify-between px-10 py-6 shrink-0 border-b border-zinc-100">
        <div className="flex items-center gap-2 text-on-surface-variant">
          <span className="text-[0.6875rem] font-medium uppercase tracking-widest opacity-60">Reports</span>
          <span className="material-symbols-outlined text-xs">chevron_right</span>
          <span className="text-[0.6875rem] font-semibold uppercase tracking-widest text-on-surface">Dossier Viewer</span>
        </div>
        <button className="px-4 py-2 text-[0.875rem] font-medium text-zinc-400 border border-zinc-200 cursor-not-allowed rounded-lg flex items-center gap-2">
          <span className="material-symbols-outlined text-sm">picture_as_pdf</span>
          Export to PDF
        </button>
      </div>

      {/* Document Viewport - Empty State */}
      <div className="flex-1 overflow-y-auto no-scrollbar flex items-center justify-center">
        <div className="text-center space-y-4 max-w-md">
           <span className="material-symbols-outlined text-zinc-200 text-6xl">search_insights</span>
           <h2 className="text-xl font-headline font-bold text-zinc-900">Nenhum Dossiê Selecionado</h2>
           <p className="text-zinc-500 text-sm">Os dossiês estratégicos gerados pelo LLM a partir dos convites de entrevista aparecerão aqui após iniciados pelo Dashboard Kanban.</p>
        </div>
      </div>
    </div>
  );
}
