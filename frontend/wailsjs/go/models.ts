export namespace folders {
	
	export interface CacheStats {
	    totalSize: number;
	    transcodedSize: number;
	    rawSegmentsSize: number;
	    metadataSize: number;
	}

}

export namespace hls {
	
	export interface FFmpegInfo {
	    ffmpegInstalled: boolean;
	    ffprobeInstalled: boolean;
	    ffmpegVersion: string;
	    ffprobeVersion: string;
	    ffmpegPath: string;
	    ffprobePath: string;
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
	    // Go type: time
	    timestamp: any;
	    deviceName: string;
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
	    defaultTranslationLanguage: string;
	    geminiApiKey: string;
	    geminiModel: string;
	    defaultQuality: string;
	    subtitleFontSize: number;
	    maxOutputWidth: number;
	    translatePromptTemplate: string;
	    maxSubtitleSamples: number;
	}
	export interface SubtitleDisplayItem {
	    path: string;
	    label: string;
	}
	export interface TrackDisplayInfo {
	    videoTracks: mediainfo.VideoTrack[];
	    audioTracks: mediainfo.AudioTrack[];
	    subtitleTracks: SubtitleDisplayItem[];
	    path: string;
	    nearSubtitle: string;
	}

}

export namespace mediainfo {
	
	export interface AudioTrack {
	    index: number;
	    language: string;
	    URI: string;
	    GroupID: string;
	    Name: string;
	    IsDefault: boolean;
	    Bandwidth: number;
	    Codecs: string;
	}
	export interface VideoTrack {
	    index: number;
	    codec: string;
	    resolution?: string;
	    URI: string;
	    GroupID: string;
	    Name: string;
	    IsDefault: boolean;
	    Bandwidth: number;
	    Codecs: string;
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
	}

}

