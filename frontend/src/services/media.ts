import type { main, options } from "../../wailsjs/go/models";

export type CastOptions = options.CastOptions;
export type PlaybackState = main.PlaybackState;
export type SubtitleDisplayItem = main.SubtitleDisplayItem;
export type SubtitleOptions = options.SubtitleCastOptions;

export const mediaService = {
  async getMediaURL(filePath: string): Promise<string> {
    const { GetMediaURL } = await import("../../wailsjs/go/main/App");
    return await GetMediaURL(filePath);
  },

  async castToDevice(
    deviceURL: string,
    mediaPath: string,
    options: CastOptions
  ): Promise<PlaybackState> {
    const { CastToDevice } = await import("../../wailsjs/go/main/App");
    return await CastToDevice(deviceURL, mediaPath, options);
  },

  async getQualityOptions(): Promise<Array<main.QualityOption>> {
    const { GetQualityOptions } = await import("../../wailsjs/go/main/App");
    return await GetQualityOptions();
  },

  async updateSubtitleSettings(options: SubtitleOptions): Promise<void> {
    const { UpdateSubtitleSettings } = await import(
      "../../wailsjs/go/main/App"
    );
    return await UpdateSubtitleSettings(options);
  },

  async getSubtitleURL(subtitlePath: string): Promise<string> {
    const { GetSubtitleURL } = await import("../../wailsjs/go/main/App");
    return await GetSubtitleURL(subtitlePath);
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
