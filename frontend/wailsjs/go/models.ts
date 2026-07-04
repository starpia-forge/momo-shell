export namespace wails {
	
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
	export class SessionInfoDTO {
	    id: string;
	    kind: string;
	    shell: string;
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
	        this.cols = source["cols"];
	        this.rows = source["rows"];
	    }
	}

}

