<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useCastStore } from '../stores/cast'
import { mediaService } from '../services/media'
import { EventsOn } from '../../wailsjs/runtime/runtime'

const castStore = useCastStore()

// Local UI state
const showSubtitleDialog = ref(false)
const subtitleTracks = ref<Array<{index: number, language: string, title: string, codec: string}>>([])

// Computed properties from store
const isCasting = computed(() => castStore.isCasting)
const playbackState = computed(() => castStore.playbackState)
const castOptions = computed(() => castStore.castOptions)

// Methods
const handleCast = async () => {
  if (!castStore.selectedDevice || !castStore.selectedMedia) return
  await castStore.startCasting()
}

const togglePlayback = async () => {
  if (playbackState.value.isPlaying) {
    if (playbackState.value.isPaused) {
      await mediaService.unpause()
    } else {
      await mediaService.pause()
    }
  }
}

const stopPlayback = async () => {
  await mediaService.stopPlayback()
}

const seek = async (seconds: number) => {
  await mediaService.seekTo(seconds)
}

const selectSubtitleFile = async () => {
  try {
    const path = await mediaService.openSubtitleDialog()
    if (path) {
      castStore.updateCastOptions({ SubtitlePath: path, SubtitleTrack: -1 })
      if (isCasting.value) {
        await mediaService.updateSubtitleSettings(castStore.castOptions)
      }
    }
  } catch (err) {
    console.error("Failed to select subtitle file", err)
  }
}

const selectSubtitleTrack = async (trackIndex: number) => {
  castStore.updateCastOptions({ SubtitleTrack: trackIndex, SubtitlePath: '' })
  if (isCasting.value) {
    await mediaService.updateSubtitleSettings(castStore.castOptions)
  }
  showSubtitleDialog.value = false
}

const loadSubtitleTracks = async () => {
  if (castStore.selectedMedia) {
    try {
      const tracks = await mediaService.getSubtitleTracks(castStore.selectedMedia)
      subtitleTracks.value = tracks
    } catch (err) {
      console.error("Failed to load subtitle tracks", err)
    }
  }
}

// Watchers
watch(() => castStore.selectedMedia, () => {
  if (castStore.selectedMedia) {
    loadSubtitleTracks()
  } else {
    subtitleTracks.value = []
  }
})

// Lifecycle
onMounted(() => {
  // Listen for playback state updates from backend
  EventsOn('playback:state', (state: any) => {
    castStore.playbackState = state
  })
  
  // Initial load
  if (castStore.selectedMedia) {
    loadSubtitleTracks()
  }
})
</script>

<template>
  <div class="media-player p-4 bg-gray-800 rounded-lg shadow-lg">
    <div v-if="castStore.isCasting" class="mb-4">
      <h3 class="text-xl font-bold text-white mb-2">Selected Media</h3>
      <p class="text-gray-300 break-all">{{ castStore.selectedMedia }}</p>
      
      <!-- Subtitle Selection -->
      <div class="mt-4">
        <h4 class="text-lg font-semibold text-white mb-2">Subtitles</h4>
        <div class="flex gap-2">
          <button 
            @click="selectSubtitleFile"
            class="px-3 py-1 bg-blue-600 text-white rounded hover:bg-blue-700 transition"
          >
            Select File
          </button>
          <button 
            @click="showSubtitleDialog = !showSubtitleDialog"
            class="px-3 py-1 bg-gray-600 text-white rounded hover:bg-gray-700 transition"
            v-if="subtitleTracks.length > 0"
          >
            Select Track
          </button>
        </div>
        
        <div v-if="castOptions.SubtitlePath" class="mt-2 text-sm text-green-400">
          Using file: {{ castOptions.SubtitlePath }}
        </div>
        <div v-if="castOptions.SubtitleTrack >= 0" class="mt-2 text-sm text-green-400">
          Using track: #{{ castOptions.SubtitleTrack }}
        </div>

        <!-- Track Selection Dialog -->
        <div v-if="showSubtitleDialog" class="mt-2 p-2 bg-gray-700 rounded">
          <div 
            v-for="track in subtitleTracks" 
            :key="track.index"
            @click="selectSubtitleTrack(track.index)"
            class="cursor-pointer p-1 hover:bg-gray-600 rounded text-sm text-gray-200"
            :class="{ 'bg-blue-900': castOptions.SubtitleTrack === track.index }"
          >
            {{ track.title || track.language || `Track ${track.index}` }} ({{ track.codec }})
          </div>
          <div 
            @click="selectSubtitleTrack(-1)"
            class="cursor-pointer p-1 hover:bg-gray-600 rounded text-sm text-gray-400 mt-1 border-t border-gray-600"
          >
            None / External File
          </div>
        </div>
      </div>
    </div>

    <div v-if="castStore.selectedDevice" class="mb-4">
      <h3 class="text-xl font-bold text-white mb-2">Device</h3>
      <p class="text-gray-300">{{ castStore.selectedDevice.name }} ({{ castStore.selectedDevice.address }})</p>
    </div>

    <div v-if="castStore.isReadyToCast" class="mt-6">
      <button 
        @click="handleCast"
        :disabled="isCasting"
        class="w-full py-3 px-6 bg-green-600 text-white font-bold rounded-lg hover:bg-green-700 transition disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center"
      >
        <span v-if="isCasting" class="mr-2 animate-spin">⟳</span>
        {{ isCasting ? 'Casting...' : 'Cast to Device' }}
      </button>
    </div>

    <div v-if="castStore.error" class="mt-4 p-3 bg-red-900/50 border border-red-700 text-red-200 rounded">
      {{ castStore.error }}
    </div>

    <!-- Playback Controls -->
    <div v-if="playbackState.isPlaying || playbackState.isPaused" class="mt-6 p-4 bg-gray-900 rounded border border-gray-700">
      <div class="flex items-center justify-between mb-2">
        <span class="text-white font-bold">{{ playbackState.mediaName }}</span>
        <span class="text-gray-400 text-sm">{{ playbackState.deviceName }}</span>
      </div>
      
      <!-- Progress Bar (Simple) -->
      <div class="w-full bg-gray-700 h-2 rounded-full mb-4 overflow-hidden">
        <div 
          class="bg-blue-500 h-full transition-all duration-1000"
          :style="{ width: `${(playbackState.currentTime / playbackState.duration) * 100}%` }"
        ></div>
      </div>
      
      <div class="flex justify-center gap-4">
        <button @click="seek(playbackState.currentTime - 30)" class="text-gray-300 hover:text-white">
          ⏪ 30s
        </button>
        <button @click="togglePlayback" class="text-white text-2xl hover:text-blue-400">
          {{ playbackState.isPaused ? '▶️' : '⏸️' }}
        </button>
        <button @click="stopPlayback" class="text-red-400 hover:text-red-300">
          ⏹️ Stop
        </button>
        <button @click="seek(playbackState.currentTime + 30)" class="text-gray-300 hover:text-white">
          30s ⏩
        </button>
      </div>
    </div>
  </div>
</template>
