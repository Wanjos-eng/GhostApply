import { useEffect, useMemo, useState } from 'react';

interface ProspectedJobDTO {
  id: string;
  titulo: string;
  empresa: string;
  url: string;
  status: string;
  fonte: string;
  descricao: string;
  criado_em: string;
}

interface ProspectedMetricsDTO {
  total_prospected: number;
  pending_count: number;
  analyzed_count: number;
  rejected_count: number;
  discarded_count: number;
  manual_count: number;
  by_source: Record<string, number>;
  by_source_last_24h: Record<string, number>;
  by_status: Record<string, number>;
}

const sourceOrder = ['linkedin', 'gupy', 'greenhouse', 'lever', 'other'];

const formatCreatedAt = (value: string) => {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return 'Unknown date';
  }
  return parsed.toLocaleString('pt-BR');
};

export function ProspectedJobsView() {
  const [jobs, setJobs] = useState<ProspectedJobDTO[]>([]);
  const [metrics, setMetrics] = useState<ProspectedMetricsDTO | null>(null);
  const [loading, setLoading] = useState(true);
  const [sourceFilter, setSourceFilter] = useState('all');

  const refresh = async () => {
    setLoading(true);
    try {
      const app = (window as any).go?.main?.App;
      if (!app) return;

      const [fetchedJobs, fetchedMetrics] = await Promise.all([
        app.FetchProspectedJobs(),
        app.GetProspectedMetrics(),
      ]);

      setJobs((fetchedJobs || []) as ProspectedJobDTO[]);
      setMetrics((fetchedMetrics || null) as ProspectedMetricsDTO | null);
    } catch (err) {
      console.error('ProspectedJobsView refresh failed:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, 15000);
    return () => clearInterval(id);
  }, []);

  const filteredJobs = useMemo(() => {
    if (sourceFilter === 'all') return jobs;
    return jobs.filter((j) => (j.fonte || 'other') === sourceFilter);
  }, [jobs, sourceFilter]);

  const sourceTabs = useMemo(() => {
    const bySource = metrics?.by_source || {};
    const ordered = sourceOrder.filter((s) => bySource[s] !== undefined);
    const remaining = Object.keys(bySource).filter((s) => !ordered.includes(s));
    return [...ordered, ...remaining];
  }, [metrics]);

  return (
    <div className="w-full h-full p-8 bg-[#f9f9f9] space-y-8">
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-6">
        <div>
          <h1 className="text-3xl font-extrabold text-[#1a1c1c] tracking-tight">Prospected Jobs</h1>
          <p className="text-zinc-500 text-sm mt-2">Operational view of collected jobs by source and pipeline status.</p>
        </div>
        <button
          onClick={refresh}
          className="px-4 py-2 text-xs font-semibold rounded-lg bg-white border border-zinc-200 hover:bg-zinc-50"
        >
          Refresh
        </button>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-6 gap-4">
        <MetricCard label="Total" value={metrics?.total_prospected ?? 0} />
        <MetricCard label="Pending" value={metrics?.pending_count ?? 0} />
        <MetricCard label="Analyzed" value={metrics?.analyzed_count ?? 0} />
        <MetricCard label="Manual" value={metrics?.manual_count ?? 0} />
        <MetricCard label="Rejected" value={metrics?.rejected_count ?? 0} />
        <MetricCard label="Discarded" value={metrics?.discarded_count ?? 0} />
      </div>

      <div className="bg-white border border-zinc-200 rounded-xl p-5">
        <div className="flex items-center justify-between gap-3 mb-4">
          <h2 className="text-sm font-extrabold uppercase tracking-wider text-zinc-700">Captured In Last 24h</h2>
          <span className="text-[11px] font-mono text-zinc-400">By source</span>
        </div>
        <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
          {sourceTabs.length === 0 ? (
            <div className="col-span-full text-xs text-zinc-400">No source metrics available yet.</div>
          ) : (
            sourceTabs.map((source) => (
              <MetricCard
                key={`last24-${source}`}
                label={source}
                value={metrics?.by_source_last_24h?.[source] || 0}
              />
            ))
          )}
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        <button
          className={`px-3 py-1.5 rounded-lg text-xs font-bold uppercase tracking-wider ${sourceFilter === 'all' ? 'bg-[#1a1c1c] text-white' : 'bg-white border border-zinc-200 text-zinc-600'}`}
          onClick={() => setSourceFilter('all')}
        >
          All ({jobs.length})
        </button>
        {sourceTabs.map((source) => (
          <button
            key={source}
            className={`px-3 py-1.5 rounded-lg text-xs font-bold uppercase tracking-wider ${sourceFilter === source ? 'bg-blue-600 text-white' : 'bg-white border border-zinc-200 text-zinc-600'}`}
            onClick={() => setSourceFilter(source)}
          >
            {source} ({metrics?.by_source?.[source] || 0})
          </button>
        ))}
      </div>

      {loading ? (
        <div className="h-48 flex items-center justify-center">
          <div className="w-8 h-8 border-4 border-indigo-500 border-t-transparent rounded-full animate-spin"></div>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {filteredJobs.length === 0 ? (
            <div className="col-span-full p-8 text-center text-zinc-400 border border-dashed border-zinc-200 rounded-lg bg-white">
              No prospected jobs for this source filter.
            </div>
          ) : (
            filteredJobs.map((job) => (
              <a
                key={job.id}
                href={job.url}
                target="_blank"
                rel="noreferrer"
                className="block p-5 bg-white border border-zinc-200 rounded-xl hover:border-zinc-400 transition"
              >
                <div className="flex items-center justify-between mb-2">
                  <span className="px-2 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-wider bg-zinc-100 text-zinc-600">
                    {job.fonte || 'other'}
                  </span>
                  <span className="text-[10px] font-mono text-zinc-400 uppercase">{job.status}</span>
                </div>
                <h3 className="font-semibold text-sm text-zinc-900 line-clamp-2">{job.titulo || 'Untitled role'}</h3>
                <p className="text-xs text-zinc-500 mt-1 truncate">{job.empresa || 'Unknown company'}</p>
                <p className="text-[11px] text-zinc-400 mt-4">{formatCreatedAt(job.criado_em)}</p>
              </a>
            ))
          )}
        </div>
      )}
    </div>
  );
}

function MetricCard({ label, value }: { label: string; value: number }) {
  return (
    <div className="bg-white p-4 rounded-lg border border-zinc-200">
      <p className="text-[10px] font-bold uppercase tracking-wider text-zinc-500">{label}</p>
      <p className="text-2xl font-mono font-bold text-zinc-900 mt-1">{value}</p>
    </div>
  );
}
