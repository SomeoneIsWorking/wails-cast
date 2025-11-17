<script setup lang="ts">
import { ref } from 'vue'
import { useCastStore } from '../stores/cast'
import { deviceService } from '../services/device'
import type { Device } from '../stores/cast'
import { Tv, RefreshCw, Cast, Check, Loader2 } from 'lucide-vue-next'

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
  <div class="card">
    <div class="card-header">
      <div class="flex items-center justify-between">
        <h2 class="text-2xl font-bold flex items-center gap-2">
          <Tv :size="28" class="text-blue-400" />
          Cast Devices
        </h2>
        <button 
          @click="handleDiscover" 
          :disabled="isRefreshing || isLoading"
          class="btn-primary flex items-center gap-2"
        >
          <RefreshCw :size="18" :class="{ 'animate-spin': isRefreshing || isLoading }" />
          {{ isRefreshing || isLoading ? 'Searching...' : 'Scan Network' }}
        </button>
      </div>
    </div>

    <div class="card-body">
      <!-- Loading State -->
      <div v-if="isRefreshing || isLoading" class="flex flex-col items-center justify-center py-12">
        <Loader2 :size="48" class="text-blue-400 mb-4 animate-spin" />
        <p class="text-gray-400">Discovering devices...</p>
      </div>

      <!-- Empty State -->
      <div v-else-if="!store.hasDevices" class="flex flex-col items-center justify-center py-12">
        <Cast :size="64" class="text-gray-600 mb-4" />
        <p class="text-gray-400 text-lg mb-2">No devices found</p>
        <p class="text-gray-500 text-sm mb-6">Make sure your Chromecast is on the same network</p>
        <button @click="handleDiscover" class="btn-primary">
          Search for Devices
        </button>
      </div>

      <!-- Device List -->
      <div v-else class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div 
          v-for="device in store.devices" 
          :key="device.url"
          @click="selectDevice(device)"
          :class="['device-item', {
            'device-item-selected': store.selectedDevice?.url === device.url
          }]"
        >
          <div class="flex items-center gap-3">
            <div class="p-3 bg-blue-600 rounded-lg">
              <Cast :size="24" />
            </div>
            <div class="flex-1 min-w-0">
              <h3 class="font-semibold text-lg truncate">{{ device.name }}</h3>
              <p class="text-sm text-gray-400 truncate">{{ device.type }}</p>
              <p class="text-xs text-gray-500 truncate">{{ device.address }}</p>
            </div>
          </div>
          <div v-if="store.selectedDevice?.url === device.url" class="shrink-0">
            <Check :size="24" class="text-blue-400" />
          </div>
        </div>
      </div>

      <!-- Device Count -->
      <div v-if="store.hasDevices && !isRefreshing && !isLoading" class="mt-6 text-center text-sm text-gray-500">
        Found {{ store.devices.length }} device{{ store.devices.length > 1 ? 's' : '' }}
      </div>
    </div>
  </div>
</template>
