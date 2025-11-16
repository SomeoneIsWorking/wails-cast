<script setup lang="ts">
import { ref } from 'vue'
import { useCastStore } from '../stores/cast'
import { deviceService } from '../services/device'
import type { Device } from '../stores/cast'
import './DeviceDiscovery.css'
import './common.css'

defineProps({
  isLoading: Boolean,
})

const emit = defineEmits<{
  select: [device: Device]
  discover: []
}>()

const store = useCastStore()
const isRefreshing = ref(false)

const handleDiscover = async () => {
  isRefreshing.value = true
  store.setLoading(true)
  store.clearError()

  try {
    const devices = await deviceService.discoverDevices()
    store.setDevices(devices)

    if (devices.length === 0) {
      store.setError('No cast devices found. Make sure devices are on the same network.')
    }
  } catch (error: unknown) {
    store.setError('Failed to discover devices')
  } finally {
    isRefreshing.value = false
    store.setLoading(false)
  }
}

const selectDevice = (device: Device) => {
  store.selectDevice(device)
  emit('select', device)
}
</script>

<template>
  <div class="device-discovery">
    <div class="discovery-header">
      <h2>🔍 Discover Devices</h2>
      <button
        class="discover-btn"
        @click="handleDiscover"
        :disabled="isRefreshing || isLoading"
      >
        <span v-if="isRefreshing || isLoading" class="spinner"></span>
        {{ isRefreshing || isLoading ? 'Discovering...' : 'Scan Network' }}
      </button>
    </div>

    <div v-if="store.hasDevices" class="devices-grid">
      <div
        v-for="device in store.devices"
        :key="device.url"
        class="device-card"
        @click="selectDevice(device)"
      >
        <div class="device-icon">📺</div>
        <div class="device-info">
          <h3>{{ device.name }}</h3>
          <p class="device-type">{{ device.type }}</p>
          <p class="device-address">{{ device.address }}</p>
        </div>
        <div class="device-arrow">→</div>
      </div>
    </div>

    <div v-else-if="!isRefreshing && !isLoading" class="empty-state">
      <div class="empty-icon">🎬</div>
      <h3>No Devices Found</h3>
      <p>Click "Scan Network" to discover Chromecast and DLNA devices on your network.</p>
    </div>
  </div>
</template>
