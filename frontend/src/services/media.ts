import { GetMediaURL, CastToDevice, GetMediaFiles, SeekTo, StopPlayback, Pause, Unpause, UpdateSubtitleSettings, GetSubtitleURL } from '../../wailsjs/go/main/App'
import type { main, mediainfo } from '../../wailsjs/go/models'

export type CastOptions = main.CastOptions;
export type MediaTrackInfo = mediainfo.MediaTrackInfo;
export type PlaybackState = main.PlaybackState;
export type SubtitleTrack = mediainfo.SubtitleTrack;

export const mediaService = {
  async getMediaURL(filePath: string): Promise<string> {
    return await GetMediaURL(filePath)
  },

  async castToDevice(deviceURL: string, mediaPath: string, options: main.CastOptions): Promise<PlaybackState> {
    return await CastToDevice(deviceURL, mediaPath, options)
  },

  async updateSubtitleSettings(options: main.CastOptions): Promise<void> {
    return await UpdateSubtitleSettings(options)
  },

  async getSubtitleURL(subtitlePath: string): Promise<string> {
    return await GetSubtitleURL(subtitlePath)
  },

  async getMediaFiles(dirPath: string): Promise<string[]> {
    return await GetMediaFiles(dirPath)
  },

  async seekTo(seekTime: number): Promise<void> {
    return await SeekTo(seekTime)
  },

  async stopPlayback(): Promise<void> {
    return await StopPlayback()
  },

  async pause(): Promise<void> {
    return await Pause()
  },

  async unpause(): Promise<void> {
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
