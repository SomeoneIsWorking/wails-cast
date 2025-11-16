export namespace main {
	
	export class Device {
	    name: string;
	    type: string;
	    url: string;
	    address: string;
	    manufacturerUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new Device(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.url = source["url"];
	        this.address = source["address"];
	        this.manufacturerUrl = source["manufacturerUrl"];
	    }
	}

}

