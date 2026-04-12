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
	export class ProfileDTO {
	    target_roles: string[];
	    core_stack: string[];
	    strictly_remote: boolean;
	    min_salary_floor: string;
	    apps_per_day: number;
	
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
	    }
	}
	export class SettingsDTO {
	    cohere_api_key: string;
	    groq_api_key: string;
	    gemini_api_key: string;
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

