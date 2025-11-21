import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { deviceService } from '../services/device'

export interface Device {
  name: string
  type: string
  url: string
  address: string
  manufacturerUrl: string
}

export const useCastStore = defineStore('cast', () => {
  // State
  const devices = ref<Device[]>([])
  const selectedDevice = ref<Device | null>(null)
  const selectedMedia = ref<string | null>(null)
  const isLoading = ref(false)
  const error = ref<string | null>(null)

  // Computed
  const hasDevices = computed(() => devices.value.length > 0)
  const hasSelectedDevice = computed(() => selectedDevice.value !== null)
  const hasSelectedMedia = computed(() => selectedMedia.value !== null)
  const isReadyToCast = computed(() => hasSelectedDevice.value && hasSelectedMedia.value)

  // Actions
  const setDevices = (newDevices: Device[]) => {
    devices.value = newDevices
  }

  const selectDevice = (device: Device) => {
    selectedDevice.value = device
    selectedMedia.value = null
  }

  const selectMedia = (mediaPath: string) => {
    selectedMedia.value = mediaPath
  }

  const setLoading = (loading: boolean) => {
    isLoading.value = loading
  }

  const setError = (errorMsg: string | null) => {
    error.value = errorMsg
  }

  const clearError = () => {
    error.value = null
  }

  const discoverDevices = async () => {
    setLoading(true);
    clearError();

    try {
      const devices = await deviceService.discoverDevices();
      setDevices(devices);
    } catch (error: unknown) {
      setError("Failed to discover devices");
      throw error;
    } finally {
      setLoading(false);
    }
  }

  const reset = () => {
    devices.value = []
    selectedDevice.value = null
    selectedMedia.value = null
    error.value = null
  }

  return {
    // State
    devices,
    selectedDevice,
    selectedMedia,
    isLoading,
    error,

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
    setError,
    clearError,
    discoverDevices,
    reset,
  }
})
