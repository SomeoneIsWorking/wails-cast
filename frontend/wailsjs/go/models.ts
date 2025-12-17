export namespace ffmpeg {
	
	export interface FFmpegInfo {
	    ffmpegInstalled: boolean;
	    ffprobeInstalled: boolean;
	    ffmpegVersion: string;
	    ffprobeVersion: string;
	    ffmpegPath: string;
	    ffprobePath: string;
	}

}

export namespace folders {
	
	export interface CacheStats {
	    totalSize: number;
	    transcodedSize: number;
	    rawSegmentsSize: number;
	    metadataSize: number;
	}

}

export namespace hls {
	
	export interface AudioTrack {
	    URI?: url.URL;
	    GroupID: string;
	    Name: string;
	    Language: string;
	    Default: boolean;
	    Autoselect: boolean;
	    Channels: string;
	    Attrs: Record<string, string>;
	    Index: number;
	}
	export interface VideoTrack {
	    URI?: url.URL;
	    Bandwidth: number;
	    Codecs: string;
	    Resolution: string;
	    FrameRate: number;
	    Audio: string;
	    Subtitles: string;
	    Attrs: Record<string, string>;
	    Index: number;
	}

}

export namespace main {
	
	export interface Device {
	    name: string;
	    type: string;
	    url: string;
	    address: string;
	    host: string;
	    port: number;
	    uuid: string;
	}
	export interface HistoryItem {
	    path: string;
	    name: string;
	    timestamp: string;
	    castOptions?: options.CastOptions;
	}
	export interface PlaybackState {
	    status: string;
	    mediaPath: string;
	    mediaName: string;
	    deviceUrl: string;
	    deviceName: string;
	    currentTime: number;
	    duration: number;
	}
	export interface Settings {
	    subtitleBurnIn: boolean;
	    ignoreClosedCaptions: boolean;
	    defaultTranslationLanguage: string;
	    geminiApiKey: string;
	    geminiModel: string;
	    defaultQuality: string;
	    subtitleFontSize: number;
	    maxOutputWidth: number;
	    translatePromptTemplate: string;
	    maxSubtitleSamples: number;
	    noTranscodeCache: boolean;
	}
	export interface SubtitleDisplayItem {
	    path: string;
	    label: string;
	}
	export interface TrackDisplayInfo {
	    videoTracks: hls.VideoTrack[];
	    audioTracks: hls.AudioTrack[];
	    subtitleTracks: SubtitleDisplayItem[];
	    path: string;
	    nearSubtitle: string;
	}

}

export namespace options {
	
	export interface CastOptions {
	    SubtitlePath: string;
	    VideoTrack: number;
	    AudioTrack: number;
	    Bitrate: string;
	}
	export interface SubtitleCastOptions {
	    Path: string;
	    BurnIn: boolean;
	    FontSize: number;
	    IgnoreClosedCaptions: boolean;
	}

}

export namespace remote {
	
	export interface DownloadStatus {
	    Status: string;
	    Segments: boolean[];
	    URL: string;
	    MediaType: string;
	    Track: number;
	}

}

export namespace url {
	
	export interface Userinfo {
	
	}
	export interface URL {
	    Scheme: string;
	    Opaque: string;
	    // Go type: Userinfo
	    User?: any;
	    Host: string;
	    Path: string;
	    RawPath: string;
	    OmitHost: boolean;
	    ForceQuery: boolean;
	    RawQuery: string;
	    Fragment: string;
	    RawFragment: string;
	}

}

