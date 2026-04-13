export namespace main {
	
	export class EmailRecrutador {
	    id: string;
	    email: string;
	    nome: string;
	    classificacao: string;
	    corpo: string;
	
	    static createFrom(source: any = {}) {
	        return new EmailRecrutador(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.email = source["email"];
	        this.nome = source["nome"];
	        this.classificacao = source["classificacao"];
	        this.corpo = source["corpo"];
	    }
	}
	export class PerformanceSuiteDTO {
	    ran_at: string;
	    samples: number;
	    database_ping_p95_ms: number;
	    database_ping_p99_ms: number;
	    database_ping_ms: number;
	    fetch_history_p95_ms: number;
	    fetch_history_p99_ms: number;
	    fetch_history_ms: number;
	    fetch_emails_p95_ms: number;
	    fetch_emails_p99_ms: number;
	    fetch_emails_ms: number;
	    fetch_interviews_p95_ms: number;
	    fetch_interviews_p99_ms: number;
	    fetch_interviews_ms: number;
	    total_suite_p95_ms: number;
	    total_suite_p99_ms: number;
	    history_rows: number;
	    email_rows: number;
	    interview_rows: number;
	    total_suite_ms: number;
	    database_reachable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PerformanceSuiteDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ran_at = source["ran_at"];
	        this.samples = source["samples"];
	        this.database_ping_p95_ms = source["database_ping_p95_ms"];
	        this.database_ping_p99_ms = source["database_ping_p99_ms"];
	        this.database_ping_ms = source["database_ping_ms"];
	        this.fetch_history_p95_ms = source["fetch_history_p95_ms"];
	        this.fetch_history_p99_ms = source["fetch_history_p99_ms"];
	        this.fetch_history_ms = source["fetch_history_ms"];
	        this.fetch_emails_p95_ms = source["fetch_emails_p95_ms"];
	        this.fetch_emails_p99_ms = source["fetch_emails_p99_ms"];
	        this.fetch_emails_ms = source["fetch_emails_ms"];
	        this.fetch_interviews_p95_ms = source["fetch_interviews_p95_ms"];
	        this.fetch_interviews_p99_ms = source["fetch_interviews_p99_ms"];
	        this.fetch_interviews_ms = source["fetch_interviews_ms"];
	        this.total_suite_p95_ms = source["total_suite_p95_ms"];
	        this.total_suite_p99_ms = source["total_suite_p99_ms"];
	        this.history_rows = source["history_rows"];
	        this.email_rows = source["email_rows"];
	        this.interview_rows = source["interview_rows"];
	        this.total_suite_ms = source["total_suite_ms"];
	        this.database_reachable = source["database_reachable"];
	    }
	}
	export class PipelineStepDTO {
	    id: string;
	    title: string;
	    status: string;
	    detail: string;
	    started_at: string;
	    finished_at: string;
	
	    static createFrom(source: any = {}) {
	        return new PipelineStepDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.detail = source["detail"];
	        this.started_at = source["started_at"];
	        this.finished_at = source["finished_at"];
	    }
	}
	export class PipelineStatusDTO {
	    state: string;
	    summary: string;
	    started_at: string;
	    updated_at: string;
	    finished_at: string;
	    steps: PipelineStepDTO[];
	    logs: string[];
	
	    static createFrom(source: any = {}) {
	        return new PipelineStatusDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.summary = source["summary"];
	        this.started_at = source["started_at"];
	        this.updated_at = source["updated_at"];
	        this.finished_at = source["finished_at"];
	        this.steps = this.convertValues(source["steps"], PipelineStepDTO);
	        this.logs = source["logs"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ProfileDTO {
	    target_roles: string[];
	    core_stack: string[];
	    strictly_remote: boolean;
	    min_salary_floor: string;
	    apps_per_day: number;
	    source_file: string;
	    parse_status: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfileDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target_roles = source["target_roles"];
	        this.core_stack = source["core_stack"];
	        this.strictly_remote = source["strictly_remote"];
	        this.min_salary_floor = source["min_salary_floor"];
	        this.apps_per_day = source["apps_per_day"];
	        this.source_file = source["source_file"];
	        this.parse_status = source["parse_status"];
	    }
	}
	export class ProspectedJobDTO {
	    id: string;
	    titulo: string;
	    empresa: string;
	    url: string;
	    status: string;
	    fonte: string;
	    descricao: string;
	    criado_em: string;
	
	    static createFrom(source: any = {}) {
	        return new ProspectedJobDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.titulo = source["titulo"];
	        this.empresa = source["empresa"];
	        this.url = source["url"];
	        this.status = source["status"];
	        this.fonte = source["fonte"];
	        this.descricao = source["descricao"];
	        this.criado_em = source["criado_em"];
	    }
	}
	export class ProspectedMetricsDTO {
	    total_prospected: number;
	    pending_count: number;
	    analyzed_count: number;
	    rejected_count: number;
	    discarded_count: number;
	    manual_count: number;
	    by_source: Record<string, number>;
	    by_source_last_24h: Record<string, number>;
	    by_status: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new ProspectedMetricsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_prospected = source["total_prospected"];
	        this.pending_count = source["pending_count"];
	        this.analyzed_count = source["analyzed_count"];
	        this.rejected_count = source["rejected_count"];
	        this.discarded_count = source["discarded_count"];
	        this.manual_count = source["manual_count"];
	        this.by_source = source["by_source"];
	        this.by_source_last_24h = source["by_source_last_24h"];
	        this.by_status = source["by_status"];
	    }
	}
	export class SettingsDTO {
	    cohere_api_key: string;
	    groq_api_key: string;
	    gemini_api_key: string;
	    ats_min_score: string;
	    imap_server: string;
	    imap_user: string;
	    imap_pass: string;
	
	    static createFrom(source: any = {}) {
	        return new SettingsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cohere_api_key = source["cohere_api_key"];
	        this.groq_api_key = source["groq_api_key"];
	        this.gemini_api_key = source["gemini_api_key"];
	        this.ats_min_score = source["ats_min_score"];
	        this.imap_server = source["imap_server"];
	        this.imap_user = source["imap_user"];
	        this.imap_pass = source["imap_pass"];
	    }
	}
	export class VagaHistoricoDTO {
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
	
	    static createFrom(source: any = {}) {
	        return new VagaHistoricoDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.vaga_id = source["vaga_id"];
	        this.titulo = source["titulo"];
	        this.empresa = source["empresa"];
	        this.url = source["url"];
	        this.vaga_status = source["vaga_status"];
	        this.candidatura_id = source["candidatura_id"];
	        this.candidatura_status = source["candidatura_status"];
	        this.recrutador_nome = source["recrutador_nome"];
	        this.recrutador_perfil = source["recrutador_perfil"];
	        this.criado_em = source["criado_em"];
	    }
	}

}

