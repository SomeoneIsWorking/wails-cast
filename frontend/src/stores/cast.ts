import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { Device, deviceService } from "../services/device";
import { CastOptions, mediaService, PlaybackState } from "../services/media";
import { EventsOn } from "../../wailsjs/runtime/runtime";

export const useCastStore = defineStore("cast", () => {
  // State
  const devices = ref<Device[]>([]);
  const selectedDevice = ref<Device | null>(null);
  const selectedMedia = ref<string | null>(null);
  const isLoading = ref(false);
  const isCasting = ref(false);
  const error = ref<string | null>(null);

  // Playback State
  const playbackState = ref<PlaybackState>({
    isPlaying: false,
    isPaused: false,
    mediaPath: "",
    mediaName: "",
    deviceUrl: "",
    deviceName: "",
    currentTime: 0,
    duration: 0,
    canSeek: false,
  });

  // Cast Options
  const castOptions = ref<CastOptions>({
    SubtitlePath: "",
    SubtitleTrack: -1,
    VideoTrack: -1,
    AudioTrack: -1,
    BurnIn: false,
    Quality: "medium",
  });

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

  const selectMedia = (mediaPath: string) => {
    selectedMedia.value = mediaPath;
  };

  const setLoading = (loading: boolean) => {
    isLoading.value = loading;
  };

  const setCasting = (casting: boolean) => {
    isCasting.value = casting;
  };

  const setError = (errorMsg: string | null) => {
    error.value = errorMsg;
  };

  const clearError = () => {
    error.value = null;
  };

  const updateCastOptions = (options: Partial<CastOptions>) => {
    castOptions.value = { ...castOptions.value, ...options };
  };

  const discoverDevices = async () => {
    setLoading(true);
    clearError();

    // Clear existing devices and register event listeners to update UI as devices are found.
    setDevices([]);
    const unsubscribers: Array<() => void> = [];

    const deviceHandler = (device: Device) => {
      // Avoid duplicates
      if (!devices.value.find((d) => d.address === device.address)) {
        setDevices([...devices.value, device]);
      }
    };
    const deviceUnsub = EventsOn("device:found", deviceHandler);
    unsubscribers.push(deviceUnsub);

    const completeHandler = () => {
      setLoading(false);
      // Unsubscribe listeners when discovery completes
      unsubscribers.forEach((u) => u());
    };
    const completeUnsub = EventsOn("discovery:complete", completeHandler);
    unsubscribers.push(completeUnsub);

    try {
      // Trigger backend discovery (returns quickly since discovery is streamed via events)
      await deviceService.discoverDevices();
    } catch (error: unknown) {
      setError("Failed to discover devices");
      // Unsubscribe if there's an error
      unsubscribers.forEach((u) => u());
      setLoading(false);
      throw error;
    }
  };

  const startCasting = async () => {
    if (!selectedDevice.value || !selectedMedia.value) return;

    setCasting(true);
    clearError();

    try {
      await mediaService.castToDevice(
        selectedDevice.value.url,
        selectedMedia.value,
        castOptions.value
      );
      // Update playback state immediately? Backend sends events/updates but we can fetch it.
      await fetchPlaybackState();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg);
    } finally {
      setCasting(false);
    }
  };

  const fetchPlaybackState = async () => {
    try {
      const state = await mediaService.getPlaybackState();
      playbackState.value = state;
    } catch (err) {
      console.error("Failed to fetch playback state", err);
    }
  };

  const reset = () => {
    devices.value = [];
    selectedDevice.value = null;
    selectedMedia.value = null;
    error.value = null;
    isCasting.value = false;
  };

  return {
    // State
    devices,
    selectedDevice,
    selectedMedia,
    isLoading,
    isCasting,
    error,
    playbackState,
    castOptions,

    // Computed
    hasDevices,
    hasSelectedDevice,
    hasSelectedMedia,
    isReadyToCast,

    // Actions
    setDevices,
    selectDevice,
    selectMedia,
    setLoading,
    setCasting,
    setError,
    clearError,
    updateCastOptions,
    discoverDevices,
    startCasting,
    fetchPlaybackState,
    reset,
  };
});
