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

  EventsOn("playback:state", (state: PlaybackState) => {
    playbackState.value = state;
    isCasting.value = playbackState.value.status !== "STOPPED";
  });

  // Playback State
  const playbackState = ref<PlaybackState>({
    status: "STOPPED",
    mediaPath: "",
    mediaName: "",
    deviceUrl: "",
    deviceName: "",
    currentTime: 0,
    duration: 0,
  });

  // Cast Options
  const castOptions = ref<CastOptions>();

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

  const startCasting = async (media: string, pCastOptions: CastOptions) => {
    if (!selectedDevice.value) return;

    castOptions.value = pCastOptions;
    selectedMedia.value = media;

    playbackState.value = await mediaService.castToDevice(
      selectedDevice.value.host,
      media,
      castOptions.value
    );
    isCasting.value = true;
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
    discoverDevices,
    startCasting,
    reset,
  };
});
