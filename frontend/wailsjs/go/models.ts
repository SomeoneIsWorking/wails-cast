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

export namespace main {
	
	export interface AppExports {
	    DownloadStatus: remote.DownloadStatus;
	}
	export interface AudioTracksDisplayItem {
	    Index: number;
	    Language: string;
	}
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
	    Path: string;
	    Label: string;
	}
	export interface VideoTrackDisplayItem {
	    Index: number;
	    Codecs: string;
	    Resolution: string;
	}
	export interface TrackDisplayInfo {
	    VideoTracks: VideoTrackDisplayItem[];
	    AudioTracks: AudioTracksDisplayItem[];
	    SubtitleTracks: SubtitleDisplayItem[];
	    Path: string;
	    NearSubtitle: string;
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
	export interface DownloadStatusQeuryResponse {
	    Status: string;
	    Segments: boolean[];
	}

}

