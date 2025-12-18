import { defineStore } from "pinia";
import { ref } from "vue";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  GetDownloadStatus,
  StartDownload,
  StopDownload,
} from "../../wailsjs/go/main/App";
import { remote } from "wailsjs/go/models";

export const useDownloadsStore = defineStore("downloads", () => {
  // State: Map with key as "url|videoTrack|audioTrack"
  const downloads = ref<Record<string, remote.DownloadStatus>>({});

  // Listen to download progress events
  EventsOn("download:progress", (data: remote.DownloadStatus) => {
    const key = `${data.URL}|${data.MediaType}|${data.Track}`;
    downloads.value[key] = data;
  });

  const startDownload = async (
    url: string,
    mediaType: string,
    track: number
  ) => {
    StartDownload(url, mediaType, track);
  };

  const stopDownload = async (
    url: string,
    mediaType: string,
    track: number
  ) => {
    await StopDownload(url, mediaType, track);
  };
  // Actions
  const getDownloadState = (url: string, mediaType: string, track: number) => {
    const key = `${url}|${mediaType}|${track}`;
    return downloads.value[key];
  };

  const loadTrackProgress = async (
    url: string,
    mediaType: string,
    track: number
  ) => {
    const status = await GetDownloadStatus(url, mediaType, track);
    downloads.value[`${url}|${mediaType}|${track}`] = {
      URL: url,
      MediaType: mediaType,
      Track: track,
      Status: status.Status,
      Segments: status.Segments,
    };
  };

  return {
    downloads,
    getDownloadState,
    loadTrackProgress,
    startDownload,
    stopDownload,
  };
});
