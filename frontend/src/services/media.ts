import { GetMediaURL, CastToDevice, GetMediaFiles } from '../../wailsjs/go/main/App'
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

  isMediaFile(filePath: string): boolean {
    const extensions = ['.mp4', '.mkv', '.webm', '.avi', '.mov', '.flv', '.m4v']
    return extensions.some(ext => filePath.toLowerCase().endsWith(ext))
  },
}
