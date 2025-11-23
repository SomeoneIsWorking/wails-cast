import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { Device, deviceService } from '../services/device'
import { EventsOn } from '../../wailsjs/runtime/runtime'

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

    // Clear existing devices and register event listeners to update UI as devices are found.
    setDevices([])
    const unsubscribers: Array<() => void> = []

    const deviceHandler = (device: Device) => {
      setDevices([...devices.value, device])
    }
    const deviceUnsub = EventsOn('device:found', deviceHandler)
    unsubscribers.push(deviceUnsub)

    const completeHandler = () => {
      setLoading(false)
      // Unsubscribe listeners when discovery completes
      unsubscribers.forEach(u => u())
    }
    const completeUnsub = EventsOn('discovery:complete', completeHandler)
    unsubscribers.push(completeUnsub)

    try {
      // Trigger backend discovery (returns quickly since discovery is streamed via events)
      await deviceService.discoverDevices();
    } catch (error: unknown) {
      setError("Failed to discover devices");
      // Unsubscribe if there's an error
      unsubscribers.forEach(u => u())
      setLoading(false)
      throw error;
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
