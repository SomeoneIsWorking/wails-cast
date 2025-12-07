export namespace folders {
	
	export class CacheStats {
	    totalSize: number;
	    transcodedSize: number;
	    rawSegmentsSize: number;
	    metadataSize: number;
	
	    static createFrom(source: any = {}) {
	        return new CacheStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalSize = source["totalSize"];
	        this.transcodedSize = source["transcodedSize"];
	        this.rawSegmentsSize = source["rawSegmentsSize"];
	        this.metadataSize = source["metadataSize"];
	    }
	}

}

export namespace hls {
	
	export class FFmpegInfo {
	    ffmpegInstalled: boolean;
	    ffprobeInstalled: boolean;
	    ffmpegVersion: string;
	    ffprobeVersion: string;
	    ffmpegPath: string;
	    ffprobePath: string;
	
	    static createFrom(source: any = {}) {
	        return new FFmpegInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ffmpegInstalled = source["ffmpegInstalled"];
	        this.ffprobeInstalled = source["ffprobeInstalled"];
	        this.ffmpegVersion = source["ffmpegVersion"];
	        this.ffprobeVersion = source["ffprobeVersion"];
	        this.ffmpegPath = source["ffmpegPath"];
	        this.ffprobePath = source["ffprobePath"];
	    }
	}

}

export namespace main {
	
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
	export class HistoryItem {
	    path: string;
	    name: string;
	    // Go type: time
	    timestamp: any;
	    deviceName: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.deviceName = source["deviceName"];
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
	export class PlaybackState {
	    status: string;
	    mediaPath: string;
	    mediaName: string;
	    deviceUrl: string;
	    deviceName: string;
	    currentTime: number;
	    duration: number;
	
	    static createFrom(source: any = {}) {
	        return new PlaybackState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.mediaPath = source["mediaPath"];
	        this.mediaName = source["mediaName"];
	        this.deviceUrl = source["deviceUrl"];
	        this.deviceName = source["deviceName"];
	        this.currentTime = source["currentTime"];
	        this.duration = source["duration"];
	    }
	}
	export class Settings {
	    subtitleBurnInDefault: boolean;
	    defaultTranslationLanguage: string;
	    geminiApiKey: string;
	    geminiModel: string;
	    defaultQuality: string;
	    subtitleFontSize: number;
	    maxOutputWidth: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.subtitleBurnInDefault = source["subtitleBurnInDefault"];
	        this.defaultTranslationLanguage = source["defaultTranslationLanguage"];
	        this.geminiApiKey = source["geminiApiKey"];
	        this.geminiModel = source["geminiModel"];
	        this.defaultQuality = source["defaultQuality"];
	        this.subtitleFontSize = source["subtitleFontSize"];
	        this.maxOutputWidth = source["maxOutputWidth"];
	    }
	}
	export class SubtitleDisplayItem {
	    path: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new SubtitleDisplayItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.label = source["label"];
	    }
	}
	export class TrackDisplayInfo {
	    videoTracks: mediainfo.VideoTrack[];
	    audioTracks: mediainfo.AudioTrack[];
	    subtitleTracks: SubtitleDisplayItem[];
	    path: string;
	    nearSubtitle: string;
	
	    static createFrom(source: any = {}) {
	        return new TrackDisplayInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.videoTracks = this.convertValues(source["videoTracks"], mediainfo.VideoTrack);
	        this.audioTracks = this.convertValues(source["audioTracks"], mediainfo.AudioTrack);
	        this.subtitleTracks = this.convertValues(source["subtitleTracks"], SubtitleDisplayItem);
	        this.path = source["path"];
	        this.nearSubtitle = source["nearSubtitle"];
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

export namespace mediainfo {
	
	export class AudioTrack {
	    index: number;
	    language: string;
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
	        this.URI = source["URI"];
	        this.GroupID = source["GroupID"];
	        this.Name = source["Name"];
	        this.IsDefault = source["IsDefault"];
	        this.Bandwidth = source["Bandwidth"];
	        this.Codecs = source["Codecs"];
	    }
	}
	export class VideoTrack {
	    index: number;
	    codec: string;
	    resolution?: string;
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
	        this.URI = source["URI"];
	        this.GroupID = source["GroupID"];
	        this.Name = source["Name"];
	        this.IsDefault = source["IsDefault"];
	        this.Bandwidth = source["Bandwidth"];
	        this.Codecs = source["Codecs"];
	    }
	}

}

export namespace options {
	
	export class SubtitleCastOptions {
	    Path: string;
	    BurnIn: boolean;
	    FontSize: number;
	
	    static createFrom(source: any = {}) {
	        return new SubtitleCastOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Path = source["Path"];
	        this.BurnIn = source["BurnIn"];
	        this.FontSize = source["FontSize"];
	    }
	}
	export class CastOptions {
	    Subtitle: SubtitleCastOptions;
	    VideoTrack: number;
	    AudioTrack: number;
	    Bitrate: string;
	    MaxOutputWidth: number;
	
	    static createFrom(source: any = {}) {
	        return new CastOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Subtitle = this.convertValues(source["Subtitle"], SubtitleCastOptions);
	        this.VideoTrack = source["VideoTrack"];
	        this.AudioTrack = source["AudioTrack"];
	        this.Bitrate = source["Bitrate"];
	        this.MaxOutputWidth = source["MaxOutputWidth"];
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

