import type { main, mediainfo } from '../../wailsjs/go/models'

export type CastOptions = main.CastOptions;
export type MediaTrackInfo = mediainfo.MediaTrackInfo;
export type PlaybackState = main.PlaybackState;
export type SubtitleTrack = mediainfo.SubtitleTrack;

export const mediaService = {
  async getMediaURL(filePath: string): Promise<string> {
    const { GetMediaURL } = await import('../../wailsjs/go/main/App')
    return await GetMediaURL(filePath)
  },

  async castToDevice(deviceURL: string, mediaPath: string, options: main.CastOptions): Promise<PlaybackState> {
    const { CastToDevice } = await import('../../wailsjs/go/main/App')
    return await CastToDevice(deviceURL, mediaPath, options)
  },

  async getQualityOptions(): Promise<Array<main.QualityOption>> {
    const { GetQualityOptions } = await import('../../wailsjs/go/main/App')
    return await GetQualityOptions()
  },

  async getDefaultQuality(): Promise<string> {
    const { GetDefaultQuality } = await import('../../wailsjs/go/main/App')
    return await GetDefaultQuality()
  },

  async updateSubtitleSettings(options: main.SubtitleOptions): Promise<void> {
    const { UpdateSubtitleSettings } = await import('../../wailsjs/go/main/App')
    return await UpdateSubtitleSettings(options)
  },

  async getSubtitleURL(subtitlePath: string): Promise<string> {
    const { GetSubtitleURL } = await import('../../wailsjs/go/main/App')
    return await GetSubtitleURL(subtitlePath)
  },

  async getMediaFiles(dirPath: string): Promise<string[]> {
    const { GetMediaFiles } = await import('../../wailsjs/go/main/App')
    return await GetMediaFiles(dirPath)
  },

  async seekTo(seekTime: number): Promise<void> {
    const { SeekTo } = await import('../../wailsjs/go/main/App')
    return await SeekTo(seekTime)
  },

  async stopPlayback(): Promise<void> {
    const { StopPlayback } = await import('../../wailsjs/go/main/App')
    return await StopPlayback()
  },

  async pause(): Promise<void> {
    const { Pause } = await import('../../wailsjs/go/main/App')
    return await Pause()
  },

  async unpause(): Promise<void> {
    const { Unpause } = await import('../../wailsjs/go/main/App')
    return await Unpause()
  },

  async openSubtitleDialog(): Promise<string> {
    const { OpenSubtitleDialog } = await import('../../wailsjs/go/main/App')
    return await OpenSubtitleDialog()
  },

  async getSubtitleTracks(videoPath: string): Promise<mediainfo.SubtitleTrack[]> {
    const { GetSubtitleTracks } = await import('../../wailsjs/go/main/App')
    return await GetSubtitleTracks(videoPath)
  },

  async getMediaTrackInfo(mediaPath: string): Promise<mediainfo.MediaTrackInfo> {
    const { GetMediaTrackInfo } = await import('../../wailsjs/go/main/App')
    return await GetMediaTrackInfo(mediaPath)
  },

  async getRemoteTrackInfo(mediaPath: string): Promise<mediainfo.MediaTrackInfo> {
    const { GetRemoteTrackInfo } = await import('../../wailsjs/go/main/App')
    return await GetRemoteTrackInfo(mediaPath)
  },

  isMediaFile(filePath: string): boolean {
    const extensions = ['.mp4', '.mkv', '.webm', '.avi', '.mov', '.flv', '.m4v']
    return extensions.some(ext => filePath.toLowerCase().endsWith(ext))
  },
}
