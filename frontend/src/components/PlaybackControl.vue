<template>
  <div v-if="playbackState.isPlaying" class="card mb-6">
    <div class="card-body">
      <!-- Header -->
      <div class="flex items-center justify-between mb-4">
        <div class="flex-1 min-w-0">
          <h3 class="text-xl font-bold truncate flex items-center gap-2">
            <Video :size="24" class="text-purple-400" />
            {{ playbackState.mediaName }}
          </h3>
          <p class="text-sm text-gray-400 truncate">
            <Cast :size="14" class="inline" />
            {{ playbackState.deviceName }}
          </p>
        </div>
        <div class="flex items-center gap-3">
          <span v-if="playbackState.isPaused" class="px-3 py-1 bg-yellow-900/30 border border-yellow-700 rounded-full text-yellow-400 text-sm font-medium flex items-center gap-1">
            <Pause :size="14" />
            Paused
          </span>
          <button @click="stopPlayback" class="btn-danger flex items-center gap-2">
            <Square :size="18" />
            Stop
          </button>
        </div>
      </div>

      <!-- Time Display -->
      <div class="flex items-center justify-between mb-2 text-sm font-mono">
        <span class="text-gray-300">{{ formatTime(playbackState.currentTime) }}</span>
        <span class="text-gray-500">{{ formatTime(playbackState.duration) }}</span>
      </div>

      <!-- Seek Bar -->
      <div class="relative mb-4">
        <input 
          type="range" 
          :min="0" 
          :max="Math.floor(playbackState.duration)" 
          v-model.number="seekPosition"
          @input="updateSeekPreview"
          @change="onSeek"
          @mousemove="updateTooltipPosition"
          @mouseenter="showTooltip = true"
          @mouseleave="showTooltip = false"
          class="seek-bar"
          ref="seekBar"
        />
        <div 
          v-if="showTooltip" 
          class="absolute bottom-full left-0 mb-2 px-2 py-1 bg-black/90 text-white text-xs rounded pointer-events-none whitespace-nowrap z-10"
          :style="{ left: tooltipPosition + 'px', transform: 'translateX(-50%)' }"
        >
          {{ formatTime(seekPreviewTime) }}
        </div>
      </div>

      <!-- Controls -->
      <div class="flex items-center justify-center gap-2">
        <button @click="seekRelative(-30)" class="btn-icon" title="Rewind 30s">
          <Rewind :size="20" />
        </button>
        <button @click="seekRelative(-10)" class="btn-icon" title="Rewind 10s">
          <SkipBack :size="20" />
        </button>
        <button 
          @click="togglePause" 
          class="btn-success px-6 py-3 flex items-center gap-2 text-lg"
        >
          <Play v-if="playbackState.isPaused" :size="20" />
          <Pause v-else :size="20" />
          {{ playbackState.isPaused ? 'Play' : 'Pause' }}
        </button>
        <button @click="seekRelative(10)" class="btn-icon" title="Forward 10s">
          <SkipForward :size="20" />
        </button>
        <button @click="seekRelative(30)" class="btn-icon" title="Forward 30s">
          <FastForward :size="20" />
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { mediaService } from '../services/media'
import { Video, Cast, Pause, Play, Square, Rewind, FastForward, SkipBack, SkipForward } from 'lucide-vue-next'

const playbackState = ref({
  isPlaying: false,
  isPaused: false,
  mediaPath: '',
  mediaName: '',
  deviceUrl: '',
  deviceName: '',
  currentTime: 0,
  duration: 0,
  canSeek: true
})

const seekPosition = ref(0)
const showTooltip = ref(false)
const tooltipPosition = ref(0)
const seekPreviewTime = ref(0)
const seekBar = ref<HTMLInputElement | null>(null)
let updateInterval: number | null = null

// Update seek preview when hovering
const updateTooltipPosition = (event: MouseEvent) => {
  if (!seekBar.value || !playbackState.value.duration) return
  
  const rect = seekBar.value.getBoundingClientRect()
  const percent = (event.clientX - rect.left) / rect.width
  const time = Math.max(0, Math.min(playbackState.value.duration, percent * playbackState.value.duration))
  
  seekPreviewTime.value = Math.floor(time)
  tooltipPosition.value = event.clientX - rect.left
}

// Update seek preview as user drags
const updateSeekPreview = () => {
  seekPreviewTime.value = seekPosition.value
}

// Load playback state
const loadPlaybackState = async () => {
  try {
    const state = await mediaService.getPlaybackState()
    playbackState.value = state
    // Sync seek position with current time from server
    seekPosition.value = Math.floor(state.currentTime)
  } catch (err) {
    console.error('Failed to load playback state:', err)
  }
}

// Toggle pause/play
const togglePause = async () => {
  try {
    if (playbackState.value.isPaused) {
      await mediaService.unpause()
    } else {
      await mediaService.pause()
    }
  } catch (err) {
    console.error('Toggle pause failed:', err)
  }
}

// Seek to position
const onSeek = async () => {
  if (!playbackState.value.canSeek) return
  
  try {
    await mediaService.seekTo(
      playbackState.value.deviceUrl,
      playbackState.value.mediaPath,
      seekPosition.value
    )
  } catch (err) {
    console.error('Seek failed:', err)
  }
}

// Seek relative
const seekRelative = async (seconds: number) => {
  const newTime = Math.max(0, Math.min(playbackState.value.duration, playbackState.value.currentTime + seconds))
  seekPosition.value = newTime
  await onSeek()
}

// Stop playback
const stopPlayback = async () => {
  try {
    await mediaService.stopPlayback()
    playbackState.value.isPlaying = false
    if (updateInterval) clearInterval(updateInterval)
  } catch (err) {
    console.error('Stop failed:', err)
  }
}

// Format time in MM:SS or HH:MM:SS
const formatTime = (seconds: number) => {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  
  if (h > 0) {
    return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
  }
  return `${m}:${s.toString().padStart(2, '0')}`
}

onMounted(() => {
  loadPlaybackState()
  
  // Poll for state updates every 2 seconds
  const pollInterval = setInterval(loadPlaybackState, 2000)
  
  onUnmounted(() => {
    clearInterval(pollInterval)
  })
})
</script>
