import { defineStore } from "pinia";
import { ref } from "vue";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  GetDownloadStatus,
  StartDownload,
  StopDownload,
} from "../../wailsjs/go/main/App";
import { download } from "../../wailsjs/go/models";

export const useDownloadsStore = defineStore("downloads", () => {
  // State: Map with key as "url|videoTrack|audioTrack"
  const downloads = ref<Record<string, download.DownloadStatus>>({});

  // Listen to download progress events
  EventsOn("download:progress", (data: download.DownloadStatus) => {
    const key = `${data.Url}|${data.MediaType}|${data.Track}`;
    downloads.value[key] = data;
  });

  const startDownload = async (
    url: string,
    mediaType: string,
    track: number
  ) => {
    const status = await StartDownload(url, mediaType, track);
    downloads.value[`${url}|${mediaType}|${track}`] = status;
  };

  const stopDownload = async (
    url: string,
    mediaType: string,
    track: number
  ) => {
    const status = await StopDownload(url, mediaType, track);
    downloads.value[`${url}|${mediaType}|${track}`] = status;
  };
  // Actions
  const getDownloadState = (
    url: string,
    mediaType: string,
    track: number
  ): download.DownloadStatus | undefined => {
    const key = `${url}|${mediaType}|${track}`;
    return downloads.value[key];
  };

  const loadTrackProgress = async (
    url: string,
    mediaType: string,
    track: number
  ) => {
    const status = await GetDownloadStatus(url, mediaType, track);
    downloads.value[`${url}|${mediaType}|${track}`] = status;
  };

  return {
    downloads,
    getDownloadState,
    loadTrackProgress,
    startDownload,
    stopDownload,
  };
});
