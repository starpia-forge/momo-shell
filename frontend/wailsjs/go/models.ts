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
	export class SharedHostDTO {
	    name: string;
	    address: string;
	    port: number;
	    labels: string[];
	    username: string;
	
	    static createFrom(source: any = {}) {
	        return new SharedHostDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.address = source["address"];
	        this.port = source["port"];
	        this.labels = source["labels"];
	        this.username = source["username"];
	    }
	}
	export class PeerViewDTO {
	    id: string;
	    name: string;
	    address: string;
	    port: number;
	    paired: boolean;
	    online: boolean;
	    lastSyncAt?: number;
	    hosts: SharedHostDTO[];
	
	    static createFrom(source: any = {}) {
	        return new PeerViewDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.address = source["address"];
	        this.port = source["port"];
	        this.paired = source["paired"];
	        this.online = source["online"];
	        this.lastSyncAt = source["lastSyncAt"];
	        this.hosts = this.convertValues(source["hosts"], SharedHostDTO);
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
	export class RemoteEntryDTO {
	    name: string;
	    path: string;
	    size: number;
	    mode: number;
	    modeText: string;
	    modTime: number;
	    isDir: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RemoteEntryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.mode = source["mode"];
	        this.modeText = source["modeText"];
	        this.modTime = source["modTime"];
	        this.isDir = source["isDir"];
	    }
	}
	export class SSHDirectSessionOpts {
	    name: string;
	    address: string;
	    port: number;
	    username: string;
	    authType: string;
	    keyPath: string;
	    secret: string;
	    cols: number;
	    rows: number;
	
	    static createFrom(source: any = {}) {
	        return new SSHDirectSessionOpts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.address = source["address"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.authType = source["authType"];
	        this.keyPath = source["keyPath"];
	        this.secret = source["secret"];
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
	export class ShareClientDTO {
	    id: string;
	    name: string;
	    pairedAt: number;
	    lastSeenAt?: number;
	
	    static createFrom(source: any = {}) {
	        return new ShareClientDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.pairedAt = source["pairedAt"];
	        this.lastSeenAt = source["lastSeenAt"];
	    }
	}
	export class ShareStatusDTO {
	    enabled: boolean;
	    pin: string;
	    port: number;
	    instanceName: string;
	    sharedHostIds: string[];
	
	    static createFrom(source: any = {}) {
	        return new ShareStatusDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.pin = source["pin"];
	        this.port = source["port"];
	        this.instanceName = source["instanceName"];
	        this.sharedHostIds = source["sharedHostIds"];
	    }
	}
	
	export class TaskInfoDTO {
	    id: string;
	    sessionId: string;
	    kind: string;
	    state: string;
	    src: string;
	    dst: string;
	    currentFile?: string;
	    bytes: number;
	    total: number;
	    offset: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new TaskInfoDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.kind = source["kind"];
	        this.state = source["state"];
	        this.src = source["src"];
	        this.dst = source["dst"];
	        this.currentFile = source["currentFile"];
	        this.bytes = source["bytes"];
	        this.total = source["total"];
	        this.offset = source["offset"];
	        this.error = source["error"];
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

