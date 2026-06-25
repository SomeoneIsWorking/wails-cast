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
	export interface CastInstance {
	    name: string;
	    host: string;
	    port: number;
	    url: string;
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
	export interface LibraryEpisode {
	    path: string;
	    name: string;
	    season: number;
	    episode: number;
	    hasSubtitles: boolean;
	    translated: boolean;
	    episodeName: string;
	    identified: boolean;
	}
	export interface LibraryItem {
	    id: string;
	    name: string;
	    path: string;
	    duration?: number;
	}
	export interface LibrarySeason {
	    name: string;
	    number: number;
	    episodes: LibraryEpisode[];
	}
	export interface LibraryShow {
	    name: string;
	    path: string;
	    seasons: LibrarySeason[];
	    tmdbId: number;
	    imdbId: string;
	    year: number;
	    identified: boolean;
	}
	export interface LibraryScanResult {
	    rootPath: string;
	    shows: LibraryShow[];
	}
	
	
	export interface OrganizeMove {
	    srcVideo: string;
	    dstVideo: string;
	    srcSubDir: string;
	    dstSubDir: string;
	    description: string;
	}
	export interface PlaybackState {
	    status: string;
	    mediaPath: string;
	    mediaName: string;
	    deviceUrl: string;
	    deviceName: string;
	    currentTime: number;
	    duration: number;
	    volume: number;
	    muted: boolean;
	}
	export interface RemoteDevice {
	    name: string;
	    host: string;
	    port: number;
	    uuid: string;
	}
	export interface RemotePlayOptions {
	    videoTrack: number;
	    audioTrack: number;
	    subtitlePath: string;
	    quality?: string;
	}
	export interface SeasonTranslateProgress {
	    showName: string;
	    seasonName: string;
	    targetLanguage: string;
	    totalEpisodes: number;
	    currentEpisode: number;
	    status: string;
	    message: string;
	}
	export interface Settings {
	    subtitleBurnIn: boolean;
	    ignoreClosedCaptions: boolean;
	    defaultTranslationLanguage: string;
	    llmProvider: string;
	    llmApiKey: string;
	    llmModel: string;
	    llmBaseURL: string;
	    defaultQuality: string;
	    subtitleFontSize: number;
	    subtitleDelaySeconds: number;
	    subtitleBold: boolean;
	    subtitleItalic: boolean;
	    maxOutputWidth: number;
	    translatePromptTemplate: string;
	    maxSubtitleSamples: number;
	    noTranscodeCache: boolean;
	    libraryRoot: string;
	    tmdbApiKey: string;
	    remoteApiEnabled: boolean;
	    remoteApiPort: number;
	    remoteApiToken: string;
	    qbtURL: string;
	    qbtUser: string;
	    qbtPass: string;
	    qbtSavePath: string;
	}
	export interface SubtitleDisplayItem {
	    Path: string;
	    Label: string;
	}
	export interface TorrentStatus {
	    hash: string;
	    name: string;
	    progress: number;
	    state: string;
	    dlspeed: number;
	    eta: number;
	    size: number;
	    content_path: string;
	    save_path: string;
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
	
	export interface translateStatus {
	    inProgress: boolean;
	    language: string;
	    files: string[];
	    error: string;
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
	    DelaySeconds: number;
	    Bold: boolean;
	    Italic: boolean;
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

