import { CastRemoteURL } from '../../wailsjs/go/main/App'
import { mediaService } from './media'

export const remoteMediaService = {
    /**
     * Cast a remote video URL directly to a Chromecast device
     * @param videoURL - The remote video URL (e.g., https://example.com/video)
     * @param deviceURL - The Chromecast device URL
     */
    async castRemoteURL(videoURL: string, deviceURL: string): Promise<void> {
        return await CastRemoteURL(videoURL, deviceURL)
    }
}

// Re-export mediaService for convenience
export { mediaService }
