<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useCastStore } from './stores/cast'
import { deviceService } from './services/device'
import DeviceDiscovery from './components/DeviceDiscovery.vue'
import MediaPlayer from './components/MediaPlayer.vue'
import FileExplorer from './components/FileExplorer.vue'
import PlaybackControl from './components/PlaybackControl.vue'
import './App.css'

const store = useCastStore()
const activeTab = ref<'devices' | 'files' | 'player'>('devices')

onMounted(async () => {
  await discoverDevices()
})

const discoverDevices = async () => {
  store.setLoading(true)
  store.clearError()

  try {
    const devices = await deviceService.discoverDevices()
    store.setDevices(devices)

    if (devices.length === 0) {
      store.setError('No cast devices found. Make sure devices are on the same network.')
    }
  } catch (error) {
    store.setError(error instanceof Error ? error.message : 'Discovery failed')
  } finally {
    store.setLoading(false)
  }
}

const selectDevice = (device: any) => {
  store.selectDevice(device)
  activeTab.value = 'files'
}

const selectMedia = () => {
  activeTab.value = 'player'
}

const handleCast = () => {
  activeTab.value = 'player'
}
</script>

<template>
  <div class="app-container">
    <header class="app-header">
      <h1>🎬 Wails Cast</h1>
      <p class="subtitle">Cast your local videos to any device</p>
    </header>

    <!-- Playback Control (shown when something is playing) -->
    <PlaybackControl />

    <main class="app-main">
      <!-- Tab Navigation -->
      <div class="tabs">
        <button 
          :class="['tab-btn', { active: activeTab === 'devices' }]"
          @click="activeTab = 'devices'"
        >
          📺 Devices
        </button>
        <button 
          v-if="store.hasSelectedDevice"
          :class="['tab-btn', { active: activeTab === 'files' }]"
          @click="activeTab = 'files'"
        >
          🎥 Media Files
        </button>
        <button 
          v-if="store.hasSelectedMedia"
          :class="['tab-btn', { active: activeTab === 'player' }]"
          @click="activeTab = 'player'"
        >
          ▶️ Cast
        </button>
      </div>

      <!-- Error Message -->
      <div v-if="store.error" class="error-banner">
        <span>⚠️ {{ store.error }}</span>
        <button @click="store.clearError" class="close-btn">✕</button>
      </div>

      <!-- Device Discovery Tab -->
      <section v-show="activeTab === 'devices'" class="tab-content">
        <DeviceDiscovery 
          :isLoading="store.isLoading"
          @discover="discoverDevices"
          @select="selectDevice"
        />
      </section>

      <!-- Media Files Tab -->
      <section v-show="activeTab === 'files'" class="tab-content">
        <div v-if="store.hasSelectedDevice" class="selected-device-info">
          <strong>Selected Device:</strong> {{ store.selectedDevice?.name }}
        </div>
        <FileExplorer 
          @select="selectMedia"
          @loading="(loading) => store.setLoading(loading)"
        />
      </section>

      <!-- Media Player Tab -->
      <section v-show="activeTab === 'player'" class="tab-content">
        <MediaPlayer 
          v-if="store.isReadyToCast && store.selectedDevice && store.selectedMedia"
          :device="store.selectedDevice"
          :mediaPath="store.selectedMedia"
          :isLoading="store.isLoading"
          @cast="handleCast"
          @back="activeTab = 'files'"
        />
      </section>
    </main>
  </div>
</template>