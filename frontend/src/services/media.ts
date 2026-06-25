import { main, options } from "../../wailsjs/go/models";
import { activeSource, isRemoteActive } from "./source";

// mediaService wraps the playback/transport bindings. When the active playback
// source is a remote instance, transport + subtitle calls are routed to that
// remote over its HTTP API; otherwise they hit the local Wails bindings.

async function remoteControl(action: string, value = 0): Promise<void> {
  const { RemoteControl } = await import("../../wailsjs/go/main/App");
  const s = activeSource.value;
  await RemoteControl(s.base, s.token, action, value);
}

export const mediaService = {
  // castToDevice is local-only here; remote playback is started by the cast
  // store via RemotePlay (it needs the remote device target).
  async castToDevice(
    deviceURL: string,
    mediaPath: string,
    options: options.CastOptions
  ): Promise<main.PlaybackState> {
    const { CastToDevice } = await import("../../wailsjs/go/main/App");
    return await CastToDevice(deviceURL, mediaPath, options);
  },

  async updateSubtitleSettings(
    options: options.SubtitleCastOptions
  ): Promise<void> {
    if (isRemoteActive()) {
      const { RemoteUpdateSubtitle } = await import("../../wailsjs/go/main/App");
      const s = activeSource.value;
      return await RemoteUpdateSubtitle(s.base, s.token, options);
    }
    const { UpdateSubtitleSettings } = await import("../../wailsjs/go/main/App");
    return await UpdateSubtitleSettings(options);
  },

  async getMediaFiles(dirPath: string): Promise<string[]> {
    const { GetMediaFiles } = await import("../../wailsjs/go/main/App");
    return await GetMediaFiles(dirPath);
  },

  async seekTo(seekTime: number): Promise<void> {
    if (isRemoteActive()) return remoteControl("seek", seekTime);
    const { SeekTo } = await import("../../wailsjs/go/main/App");
    return await SeekTo(seekTime);
  },

  async stopPlayback(): Promise<void> {
    if (isRemoteActive()) return remoteControl("stop");
    const { StopPlayback } = await import("../../wailsjs/go/main/App");
    return await StopPlayback();
  },

  async pause(): Promise<void> {
    if (isRemoteActive()) return remoteControl("pause");
    const { Pause } = await import("../../wailsjs/go/main/App");
    return await Pause();
  },

  async unpause(): Promise<void> {
    if (isRemoteActive()) return remoteControl("resume");
    const { Unpause } = await import("../../wailsjs/go/main/App");
    return await Unpause();
  },

  async setVolume(value: number): Promise<void> {
    if (isRemoteActive()) return remoteControl("volume", value);
    const { SetVolume } = await import("../../wailsjs/go/main/App");
    return await SetVolume(value);
  },

  async setMuted(muted: boolean): Promise<void> {
    if (isRemoteActive()) return remoteControl(muted ? "mute" : "unmute");
    const { SetMuted } = await import("../../wailsjs/go/main/App");
    return await SetMuted(muted);
  },

  isMediaFile(filePath: string): boolean {
    const extensions = [
      ".mp4",
      ".mkv",
      ".webm",
      ".avi",
      ".mov",
      ".flv",
      ".m4v",
    ];
    return extensions.some((ext) => filePath.toLowerCase().endsWith(ext));
  },
};
