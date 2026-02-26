export namespace main {
	
	export class ServiceStatus {
	    sensoryRunning: boolean;
	    nextjsRunning: boolean;
	    sensoryPort: number;
	    nextjsPort: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ServiceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sensoryRunning = source["sensoryRunning"];
	        this.nextjsRunning = source["nextjsRunning"];
	        this.sensoryPort = source["sensoryPort"];
	        this.nextjsPort = source["nextjsPort"];
	        this.message = source["message"];
	    }
	}

}

