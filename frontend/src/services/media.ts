import { GetMediaURL, CastToDevice, GetMediaFiles, SeekTo, GetPlaybackState, UpdatePlaybackState, StopPlayback, Pause, Unpause, UpdateSubtitleSettings, GetSubtitleURL } from '../../wailsjs/go/main/App'
import type { main } from '../../wailsjs/go/models'

export const mediaService = {
  async getMediaURL(filePath: string): Promise<string> {
    return await GetMediaURL(filePath)
  },

  async castToDevice(deviceURL: string, mediaPath: string, subtitlePath?: string, subtitleTrack?: number): Promise<void> {
    const options: main.CastOptions = {
      SubtitlePath: subtitlePath || '',
      SubtitleTrack: subtitleTrack ?? -1
    }
    return await CastToDevice(deviceURL, mediaPath, options)
  },

  async updateSubtitleSettings(subtitlePath?: string, subtitleTrack?: number): Promise<void> {
    const options: main.CastOptions = {
      SubtitlePath: subtitlePath || '',
      SubtitleTrack: subtitleTrack ?? -1
    }
    return await UpdateSubtitleSettings(options)
  },

  async getSubtitleURL(subtitlePath: string): Promise<string> {
    return await GetSubtitleURL(subtitlePath)
  },

  async getMediaFiles(dirPath: string): Promise<string[]> {
    return await GetMediaFiles(dirPath)
  },

  async seekTo(deviceURL: string, mediaPath: string, seekTime: number): Promise<void> {
    return await SeekTo(deviceURL, mediaPath, seekTime)
  },

  async getPlaybackState(): Promise<main.PlaybackState> {
    return await GetPlaybackState()
  },

  async updatePlaybackState(): Promise<main.PlaybackState> {
    return await UpdatePlaybackState()
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

  isMediaFile(filePath: string): boolean {
    const extensions = ['.mp4', '.mkv', '.webm', '.avi', '.mov', '.flv', '.m4v']
    return extensions.some(ext => filePath.toLowerCase().endsWith(ext))
  },
}
