import { DiscoverDevices, GetLocalIP } from '../../wailsjs/go/main/App'
import type { Device } from '../stores/cast'

export const deviceService = {
  async discoverDevices(): Promise<Device[]> {
    const devices = await DiscoverDevices()
    return devices || []
  },

  async getLocalIP(): Promise<string> {
    return await GetLocalIP()
  },
}
