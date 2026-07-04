export namespace wails {
	
	export class HistoryEntryDTO {
	    id: number;
	    hostId?: string;
	    command: string;
	    executedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new HistoryEntryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.hostId = source["hostId"];
	        this.command = source["command"];
	        this.executedAt = source["executedAt"];
	    }
	}
	export class HistoryQueryDTO {
	    hostId: string;
	    search: string;
	    limit: number;
	    offset: number;
	
	    static createFrom(source: any = {}) {
	        return new HistoryQueryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hostId = source["hostId"];
	        this.search = source["search"];
	        this.limit = source["limit"];
	        this.offset = source["offset"];
	    }
	}
	export class HostDTO {
	    id: string;
	    name: string;
	    address: string;
	    port: number;
	    labels: string[];
	    username: string;
	    authType: string;
	    keyPath?: string;
	    source: string;
	    createdAt: number;
	    updatedAt: number;
	    lastConnectedAt?: number;
	
	    static createFrom(source: any = {}) {
	        return new HostDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.address = source["address"];
	        this.port = source["port"];
	        this.labels = source["labels"];
	        this.username = source["username"];
	        this.authType = source["authType"];
	        this.keyPath = source["keyPath"];
	        this.source = source["source"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.lastConnectedAt = source["lastConnectedAt"];
	    }
	}
	export class HostInputDTO {
	    id: string;
	    name: string;
	    address: string;
	    port: number;
	    labels: string[];
	    username: string;
	    authType: string;
	    keyPath: string;
	
	    static createFrom(source: any = {}) {
	        return new HostInputDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.address = source["address"];
	        this.port = source["port"];
	        this.labels = source["labels"];
	        this.username = source["username"];
	        this.authType = source["authType"];
	        this.keyPath = source["keyPath"];
	    }
	}
	export class LocalSessionOpts {
	    shell: string;
	    cwd: string;
	    cols: number;
	    rows: number;
	
	    static createFrom(source: any = {}) {
	        return new LocalSessionOpts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shell = source["shell"];
	        this.cwd = source["cwd"];
	        this.cols = source["cols"];
	        this.rows = source["rows"];
	    }
	}
	export class SSHSessionOpts {
	    hostId: string;
	    cols: number;
	    rows: number;
	
	    static createFrom(source: any = {}) {
	        return new SSHSessionOpts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hostId = source["hostId"];
	        this.cols = source["cols"];
	        this.rows = source["rows"];
	    }
	}
	export class SessionInfoDTO {
	    id: string;
	    kind: string;
	    shell?: string;
	    hostId?: string;
	    cols: number;
	    rows: number;
	
	    static createFrom(source: any = {}) {
	        return new SessionInfoDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.shell = source["shell"];
	        this.hostId = source["hostId"];
	        this.cols = source["cols"];
	        this.rows = source["rows"];
	    }
	}
	export class TestResultDTO {
	    stage: string;
	    ok: boolean;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new TestResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stage = source["stage"];
	        this.ok = source["ok"];
	        this.message = source["message"];
	    }
	}

}

