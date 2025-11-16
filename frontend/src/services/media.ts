import { GetMediaURL, CastToDevice, GetMediaFiles } from '../../wailsjs/go/main/App'

export const mediaService = {
  async getMediaURL(filePath: string): Promise<string> {
    return await GetMediaURL(filePath)
  },

  async castToDevice(deviceURL: string, mediaPath: string): Promise<void> {
    return await CastToDevice(deviceURL, mediaPath)
  },

  async getMediaFiles(dirPath: string): Promise<string[]> {
    return await GetMediaFiles(dirPath)
  },

  isMediaFile(filePath: string): boolean {
    const extensions = ['.mp4', '.mkv', '.webm', '.avi', '.mov', '.flv', '.m4v']
    return extensions.some(ext => filePath.toLowerCase().endsWith(ext))
  },
}
