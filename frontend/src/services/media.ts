import { GetMediaURL, CastToDevice, GetMediaFiles, SeekTo, GetPlaybackState, StopPlayback, Pause, Unpause } from '../../wailsjs/go/main/App'
import type { main } from '../../wailsjs/go/models'

export const mediaService = {
  async getMediaURL(filePath: string): Promise<string> {
    return await GetMediaURL(filePath)
  },

  async castToDevice(deviceURL: string, mediaPath: string, subtitlePath?: string): Promise<void> {
    const options: main.CastOptions = {
      subtitlePath: subtitlePath || ''
    }
    return await CastToDevice(deviceURL, mediaPath, options)
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
