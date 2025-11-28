import { DiscoverDevices } from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";

export type Device = main.Device;

export const deviceService = {
  async discoverDevices(): Promise<main.Device[]> {
    const devices = await DiscoverDevices();
    return devices || [];
  },
};
