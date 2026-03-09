import { main, options } from "../../wailsjs/go/models";

export const mediaService = {
  async castToDevice(
    deviceURL: string,
    mediaPath: string,
    options: options.CastOptions
  ): Promise<main.PlaybackState> {
    const { CastToDevice } = await import("../../wailsjs/go/main/App");
    return await CastToDevice(deviceURL, mediaPath, options);
  },

  async updateSubtitleSettings(options: options.SubtitleCastOptions): Promise<void> {
    const { UpdateSubtitleSettings } = await import(
      "../../wailsjs/go/main/App"
    );
    return await UpdateSubtitleSettings(options);
  },

  async getMediaFiles(dirPath: string): Promise<string[]> {
    const { GetMediaFiles } = await import("../../wailsjs/go/main/App");
    return await GetMediaFiles(dirPath);
  },

  async seekTo(seekTime: number): Promise<void> {
    const { SeekTo } = await import("../../wailsjs/go/main/App");
    return await SeekTo(seekTime);
  },

  async stopPlayback(): Promise<void> {
    const { StopPlayback } = await import("../../wailsjs/go/main/App");
    return await StopPlayback();
  },

  async pause(): Promise<void> {
    const { Pause } = await import("../../wailsjs/go/main/App");
    return await Pause();
  },

  async unpause(): Promise<void> {
    const { Unpause } = await import("../../wailsjs/go/main/App");
    return await Unpause();
  },

  async setVolume(value: number): Promise<void> {
    const { SetVolume } = await import("../../wailsjs/go/main/App");
    return await SetVolume(value);
  },

  async setMuted(muted: boolean): Promise<void> {
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
