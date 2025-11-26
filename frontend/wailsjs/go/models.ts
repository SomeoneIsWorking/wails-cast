export namespace main {
	
	export class CastOptions {
	    SubtitlePath: string;
	    SubtitleTrack: number;
	    VideoTrack: number;
	    AudioTrack: number;
	    BurnIn: boolean;
	    Quality: string;
	
	    static createFrom(source: any = {}) {
	        return new CastOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.SubtitlePath = source["SubtitlePath"];
	        this.SubtitleTrack = source["SubtitleTrack"];
	        this.VideoTrack = source["VideoTrack"];
	        this.AudioTrack = source["AudioTrack"];
	        this.BurnIn = source["BurnIn"];
	        this.Quality = source["Quality"];
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

}

export namespace mediainfo {
	
	export class AudioTrack {
	    index: number;
	    language: string;
	    codec: string;
	    Type: string;
	    URI: string;
	    GroupID: string;
	    Name: string;
	    IsDefault: boolean;
	    Bandwidth: number;
	    Codecs: string;
	
	    static createFrom(source: any = {}) {
	        return new AudioTrack(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.language = source["language"];
	        this.codec = source["codec"];
	        this.Type = source["Type"];
	        this.URI = source["URI"];
	        this.GroupID = source["GroupID"];
	        this.Name = source["Name"];
	        this.IsDefault = source["IsDefault"];
	        this.Bandwidth = source["Bandwidth"];
	        this.Codecs = source["Codecs"];
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
	export class VideoTrack {
	    index: number;
	    codec: string;
	    resolution?: string;
	    Type: string;
	    URI: string;
	    GroupID: string;
	    Name: string;
	    IsDefault: boolean;
	    Bandwidth: number;
	    Codecs: string;
	
	    static createFrom(source: any = {}) {
	        return new VideoTrack(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.codec = source["codec"];
	        this.resolution = source["resolution"];
	        this.Type = source["Type"];
	        this.URI = source["URI"];
	        this.GroupID = source["GroupID"];
	        this.Name = source["Name"];
	        this.IsDefault = source["IsDefault"];
	        this.Bandwidth = source["Bandwidth"];
	        this.Codecs = source["Codecs"];
	    }
	}
	export class MediaTrackInfo {
	    videoTracks: VideoTrack[];
	    audioTracks: AudioTrack[];
	    subtitleTracks: SubtitleTrack[];
	
	    static createFrom(source: any = {}) {
	        return new MediaTrackInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.videoTracks = this.convertValues(source["videoTracks"], VideoTrack);
	        this.audioTracks = this.convertValues(source["audioTracks"], AudioTrack);
	        this.subtitleTracks = this.convertValues(source["subtitleTracks"], SubtitleTrack);
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
	

}

