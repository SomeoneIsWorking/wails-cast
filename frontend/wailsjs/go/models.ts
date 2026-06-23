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
	export interface LibraryEpisode {
	    path: string;
	    name: string;
	    season: number;
	    episode: number;
	    hasSubtitles: boolean;
	    /** Official episode title from TMDB (empty if unidentified). */
	    episodeName: string;
	    /** True when TMDB metadata was successfully fetched. */
	    identified: boolean;
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
	    /** TMDB series ID (0 if unidentified). */
	    tmdbId: number;
	    /** IMDb ID string, e.g. "tt1234567" (empty if unidentified). */
	    imdbId: string;
	    /** First-air-date year from TMDB (0 if unidentified). */
	    year: number;
	    /** True when TMDB metadata was successfully fetched. */
	    identified: boolean;
	}
	export interface LibraryScanResult {
	    rootPath: string;
	    shows: LibraryShow[];
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
	export interface LibraryIdentifyProgress {
	    total: number;
	    current: number;
	    showName: string;
	    status: string;
	    message: string;
	}
	export interface OrganizeMove {
	    srcVideo: string;
	    dstVideo: string;
	    /** Non-empty when a sibling subtitle directory will also be moved. */
	    srcSubDir: string;
	    dstSubDir: string;
	    description: string;
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
	    volume: number;
	    muted: boolean;
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
	    maxOutputWidth: number;
	    translatePromptTemplate: string;
	    maxSubtitleSamples: number;
	    noTranscodeCache: boolean;
	    libraryRoot: string;
	    /** TMDB v3 API key for show/episode identification. */
	    tmdbApiKey: string;
	    remoteApiEnabled: boolean;
	    remoteApiPort: number;
	    remoteApiToken: string;
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

