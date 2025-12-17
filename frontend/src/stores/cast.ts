import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { Device, deviceService } from "../services/device";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { ffmpeg, main, options } from "../../wailsjs/go/models";
import { mediaService } from "@/services/media";
import { useSettingsStore } from "./settings";
import { parseSubtitlePath, buildSubtitlePath } from "@/utils/subtitle";
import { useHistoryStore } from "./history";

interface FrontendCastOptions {
  VideoTrack: number;
  AudioTrack: number;
  Bitrate: string;
  SubtitleType: string;
  SubtitlePath: string;
}

export const useCastStore = defineStore("cast", () => {
  const settingsStore = useSettingsStore();

  // State
  const devices = ref<Device[]>([]);
  const selectedDevice = ref<Device | null>(null);
  const selectedMedia = ref<string | null>(null);
  const isLoading = ref(false);
  const isCasting = computed(() => playbackState.value.status !== "STOPPED");
  const error = ref<string | null>(null);
  const ffmpegInfo = ref<ffmpeg.FFmpegInfo | null>(null);
  const trackInfo = ref<main.TrackDisplayInfo | null>(null);
  const historyStore = useHistoryStore();

  EventsOn("playback:state", (state: main.PlaybackState) => {
    playbackState.value = state;
  });

  // Playback State
  const playbackState = ref<main.PlaybackState>({
    status: "STOPPED",
    mediaPath: "",
    mediaName: "",
    deviceUrl: "",
    deviceName: "",
    currentTime: 0,
    duration: 0,
  });

  // Cast Options
  const castOptions = ref<FrontendCastOptions>();

  // Computed
  const hasDevices = computed(() => devices.value.length > 0);
  const hasSelectedDevice = computed(() => selectedDevice.value !== null);
  const hasSelectedMedia = computed(() => selectedMedia.value !== null);
  const isReadyToCast = computed(
    () => hasSelectedDevice.value && hasSelectedMedia.value
  );

  // Actions
  const setDevices = (newDevices: Device[]) => {
    devices.value = newDevices;
  };

  const selectDevice = (device: Device) => {
    selectedDevice.value = device;
    // Reset media if needed, or keep it
  };

  const setTrackInfo = (info: main.TrackDisplayInfo) => {
    trackInfo.value = info;
    const historyItem = historyStore.items.find(
      (item) => item.path === info.path
    );

    const historyCastOptions = historyItem?.castOptions;
    if (historyCastOptions) {
      const subtitleItem = parseSubtitlePath(
        historyCastOptions.SubtitlePath
      );
      castOptions.value = {
        VideoTrack: historyCastOptions.VideoTrack,
        AudioTrack: historyCastOptions.AudioTrack,
        Bitrate: historyCastOptions.Bitrate,
        SubtitleType: subtitleItem.type,
        SubtitlePath: subtitleItem.path,
      };
    } else {
      castOptions.value = {
        VideoTrack: 0,
        AudioTrack: 0,
        Bitrate: settingsStore.settings.defaultQuality,
        SubtitleType: info?.nearSubtitle ? "external" : "none",
        SubtitlePath: info?.nearSubtitle || "",
      };
    }
  };

  EventsOn("discovery:complete", () => {
    isLoading.value = false;
  });
  EventsOn("device:found", (device: Device) => {
    // Avoid duplicates
    if (!devices.value.find((d) => d.address === device.address)) {
      setDevices([...devices.value, device]);
    }
  });

  const discoverDevices = async () => {
    isLoading.value = true;
    // Clear existing devices and register event listeners to update UI as devices are found.
    setDevices([]);

    // Trigger backend discovery (returns quickly since discovery is streamed via events)
    await deviceService.discoverDevices();
  };

  const startCasting = async (media: string) => {
    if (!selectedDevice.value || !castOptions.value) return;

    selectedMedia.value = media;

    const backendOptions: options.CastOptions = {
      VideoTrack: castOptions.value.VideoTrack,
      AudioTrack: castOptions.value.AudioTrack,
      Bitrate: castOptions.value.Bitrate,
      SubtitlePath: buildSubtitlePath(
        castOptions.value.SubtitleType,
        castOptions.value.SubtitlePath
      ),
    };

    playbackState.value = await mediaService.castToDevice(
      selectedDevice.value.host,
      media,
      backendOptions
    );
  };

  const checkFFmpeg = async () => {
    const { GetFFmpegInfo } = await import("../../wailsjs/go/main/App");
    ffmpegInfo.value = await GetFFmpegInfo(true);
  };

  return {
    // State
    devices,
    selectedDevice,
    selectedMedia,
    isLoading,
    isCasting,
    error,
    ffmpegInfo,
    playbackState,
    castOptions,
    trackInfo,

    // Computed
    hasDevices,
    hasSelectedDevice,
    hasSelectedMedia,
    isReadyToCast,

    // Actions
    setDevices,
    selectDevice,
    setTrackInfo,
    discoverDevices,
    startCasting,
    checkFFmpeg,
  };
});
