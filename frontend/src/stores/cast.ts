import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { Device, deviceService } from "../services/device";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { ffmpeg, main, options } from "../../wailsjs/go/models";
import { mediaService } from "@/services/media";
import { useSettingsStore } from "./settings";
import { parseSubtitlePath, buildSubtitlePath } from "@/utils/subtitle";
import { useHistoryStore } from "./history";
import { activeSource, isRemoteActive, type Source } from "@/services/source";

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

  // Cast targets exposed by the active remote source (when playing a remote
  // item), and the chosen target host on that remote.
  const remoteDevices = ref<main.RemoteDevice[]>([]);
  const remoteTargetHost = ref<string>("local");

  // Local playback events only apply when playing locally; remote playback is
  // reflected by polling the remote /state (see startStatePoll).
  EventsOn("playback:state", (state: main.PlaybackState) => {
    if (isRemoteActive()) return;
    playbackState.value = state;
  });

  // Poll the active remote instance's playback state while it is the target.
  let statePoll: number | null = null;
  const stopStatePoll = () => {
    if (statePoll !== null) {
      clearInterval(statePoll);
      statePoll = null;
    }
  };
  const startStatePoll = () => {
    stopStatePoll();
    statePoll = window.setInterval(async () => {
      if (!isRemoteActive()) return;
      try {
        const { RemoteState } = await import("../../wailsjs/go/main/App");
        const s = activeSource.value;
        playbackState.value = await RemoteState(s.base, s.token);
      } catch {
        /* transient */
      }
    }, 2000);
  };

  // Playback State
  const playbackState = ref<main.PlaybackState>({
    status: "STOPPED",
    mediaPath: "",
    mediaName: "",
    deviceUrl: "",
    deviceName: "",
    currentTime: 0,
    duration: 0,
    volume: 1,
    muted: false,
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
      (item) => item.path === info.Path
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
        SubtitleType: info?.NearSubtitle ? "external" : "none",
        SubtitlePath: info?.NearSubtitle || "",
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

  // prepareEpisode loads track info for an item from a given source, sets it as
  // the active playback source, and (for remote sources) loads that source's
  // cast targets. The UI then shows Cast Options to start playback.
  const prepareEpisode = async (path: string, source: Source) => {
    activeSource.value = source;
    if (source.kind === "remote") {
      const { RemoteTrackInfo, RemoteDevices } = await import(
        "../../wailsjs/go/main/App"
      );
      const [info, devs] = await Promise.all([
        RemoteTrackInfo(source.base, source.token, path),
        RemoteDevices(source.base, source.token),
      ]);
      remoteDevices.value = devs;
      // Default to the first real cast target (Chromecast), else the remote itself.
      const def = devs.find((d) => d.host !== "local") || devs[0];
      remoteTargetHost.value = def ? def.host : "local";
      setTrackInfo(info);
    } else {
      const { GetTrackDisplayInfo } = await import("../../wailsjs/go/main/App");
      setTrackInfo(await GetTrackDisplayInfo(path));
    }
  };

  const startCasting = async (media: string) => {
    if (!castOptions.value) return;
    selectedMedia.value = media;

    const subtitlePath = buildSubtitlePath(
      castOptions.value.SubtitleType,
      castOptions.value.SubtitlePath
    );

    if (isRemoteActive()) {
      const { RemotePlay } = await import("../../wailsjs/go/main/App");
      const s = activeSource.value;
      playbackState.value = await RemotePlay(
        s.base,
        s.token,
        media,
        remoteTargetHost.value || "local",
        {
          videoTrack: castOptions.value.VideoTrack,
          audioTrack: castOptions.value.AudioTrack,
          subtitlePath,
          quality: castOptions.value.Bitrate,
        }
      );
      startStatePoll();
    } else {
      if (!selectedDevice.value) return;
      const backendOptions: options.CastOptions = {
        VideoTrack: castOptions.value.VideoTrack,
        AudioTrack: castOptions.value.AudioTrack,
        Bitrate: castOptions.value.Bitrate,
        SubtitlePath: subtitlePath,
      };
      playbackState.value = await mediaService.castToDevice(
        selectedDevice.value.host,
        media,
        backendOptions
      );
    }

    // Re-apply this episode's saved subtitle timing offset, if any.
    const savedDelay = getSubtitleDelay(media);
    if (savedDelay !== 0) {
      await updateLiveSubtitleSettings({ delaySeconds: savedDelay });
    }
  };

  const checkFFmpeg = async () => {
    const { GetFFmpegInfo } = await import("../../wailsjs/go/main/App");
    ffmpegInfo.value = await GetFFmpegInfo(true);
  };

  // Subtitle timing offset is remembered PER EPISODE (keyed by the media path)
  // because the right sync differs per file; size and style stay global.
  const SUB_DELAY_PREFIX = "subDelay:";
  const getSubtitleDelay = (mediaPath: string): number => {
    if (!mediaPath) return 0;
    const raw = localStorage.getItem(SUB_DELAY_PREFIX + mediaPath);
    const v = raw ? parseFloat(raw) : 0;
    return Number.isFinite(v) ? v : 0;
  };
  const setSubtitleDelay = (mediaPath: string, delay: number) => {
    if (!mediaPath) return;
    const key = SUB_DELAY_PREFIX + mediaPath;
    if (delay === 0) localStorage.removeItem(key);
    else localStorage.setItem(key, String(delay));
  };

  // updateLiveSubtitleSettings applies subtitle tweaks (timing offset, bold,
  // italic, font size) to the running playback without recasting. Size/style
  // overrides persist to global settings; the timing offset persists per episode.
  const updateLiveSubtitleSettings = async (overrides: {
    delaySeconds?: number;
    bold?: boolean;
    italic?: boolean;
    fontSize?: number;
  }) => {
    const settings = settingsStore.settings;
    const mediaPath = playbackState.value.mediaPath;

    // Persist size/style globally so they stick across casts.
    if (settings && (overrides.bold !== undefined || overrides.italic !== undefined || overrides.fontSize !== undefined)) {
      const next: main.Settings = { ...settings };
      if (overrides.bold !== undefined) next.subtitleBold = overrides.bold;
      if (overrides.italic !== undefined) next.subtitleItalic = overrides.italic;
      if (overrides.fontSize !== undefined)
        next.subtitleFontSize = overrides.fontSize;
      await settingsStore.saveSettings(next);
    }
    // Persist the timing offset per episode.
    if (overrides.delaySeconds !== undefined) {
      setSubtitleDelay(mediaPath, overrides.delaySeconds);
    }

    const path = castOptions.value
      ? buildSubtitlePath(
          castOptions.value.SubtitleType,
          castOptions.value.SubtitlePath
        )
      : "none";

    const opts: options.SubtitleCastOptions = {
      Path: path,
      BurnIn: settings?.subtitleBurnIn ?? false,
      FontSize: overrides.fontSize ?? settings?.subtitleFontSize ?? 24,
      IgnoreClosedCaptions: settings?.ignoreClosedCaptions ?? false,
      DelaySeconds: overrides.delaySeconds ?? getSubtitleDelay(mediaPath),
      Bold: overrides.bold ?? settings?.subtitleBold ?? false,
      Italic: overrides.italic ?? settings?.subtitleItalic ?? false,
    };

    await mediaService.updateSubtitleSettings(opts);
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
    remoteDevices,
    remoteTargetHost,

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
    prepareEpisode,
    startCasting,
    checkFFmpeg,
    updateLiveSubtitleSettings,
    getSubtitleDelay,
  };
});
