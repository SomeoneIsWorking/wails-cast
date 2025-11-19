export namespace main {
	
	export class CastOptions {
	    SubtitlePath: string;
	    SubtitleTrack: number;
	
	    static createFrom(source: any = {}) {
	        return new CastOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.SubtitlePath = source["SubtitlePath"];
	        this.SubtitleTrack = source["SubtitleTrack"];
	    }
	}
	export class Device {
	    name: string;
	    type: string;
	    url: string;
	    address: string;
	    host: string;
	    port: number;
	    uuid: string;
	
	    static createFrom(source: any = {}) {
	        return new Device(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.url = source["url"];
	        this.address = source["address"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.uuid = source["uuid"];
	    }
	}
	export class PlaybackState {
	    isPlaying: boolean;
	    isPaused: boolean;
	    mediaPath: string;
	    mediaName: string;
	    deviceUrl: string;
	    deviceName: string;
	    currentTime: number;
	    duration: number;
	    canSeek: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PlaybackState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isPlaying = source["isPlaying"];
	        this.isPaused = source["isPaused"];
	        this.mediaPath = source["mediaPath"];
	        this.mediaName = source["mediaName"];
	        this.deviceUrl = source["deviceUrl"];
	        this.deviceName = source["deviceName"];
	        this.currentTime = source["currentTime"];
	        this.duration = source["duration"];
	        this.canSeek = source["canSeek"];
	    }
	}
	export class SubtitleTrack {
	    index: number;
	    language: string;
	    title: string;
	    codec: string;
	
	    static createFrom(source: any = {}) {
	        return new SubtitleTrack(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.language = source["language"];
	        this.title = source["title"];
	        this.codec = source["codec"];
	    }
	}

}

