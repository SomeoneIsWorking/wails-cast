<template>
  <div v-if="playbackState.isPlaying" class="playback-control">
    <div class="playback-header">
      <h3>{{ playbackState.mediaName }}</h3>
      <button @click="stopPlayback" class="stop-btn">Stop</button>
    </div>
    
    <div class="playback-info">
      <span class="device-name">{{ playbackState.deviceName }}</span>
      <span class="time-display">{{ formatTime(currentTime) }} / {{ formatTime(playbackState.duration) }}</span>
    </div>

    <div class="seek-controls">
      <div class="seek-container">
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
          class="seek-tooltip"
          :style="{ left: tooltipPosition + 'px' }"
        >
          {{ formatTime(seekPreviewTime) }}
        </div>
      </div>
    </div>

    <div class="playback-controls">
      <button @click="seekRelative(-30)" class="control-btn">-30s</button>
      <button @click="seekRelative(-10)" class="control-btn">-10s</button>
      <button @click="seekRelative(10)" class="control-btn">+10s</button>
      <button @click="seekRelative(30)" class="control-btn">+30s</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { GetPlaybackState, SeekTo, StopPlayback } from '../../wailsjs/go/main/App'

const playbackState = ref({
  isPlaying: false,
  mediaPath: '',
  mediaName: '',
  deviceUrl: '',
  deviceName: '',
  currentTime: 0,
  duration: 0,
  canSeek: true
})

const seekPosition = ref(0)
const currentTime = ref(0)
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

// Watch currentTime and update seekPosition to sync with playback
watch(currentTime, (newTime) => {
  seekPosition.value = newTime
})

// Load playback state
const loadPlaybackState = async () => {
  try {
    const state = await GetPlaybackState()
    playbackState.value = state
    // Initialize position from the state's current time (which includes seek offset)
    seekPosition.value = state.currentTime
    currentTime.value = state.currentTime
  } catch (err) {
    console.error('Failed to load playback state:', err)
  }
}

// Update current time (simulate playback progress)
const startTimeUpdate = () => {
  if (updateInterval) clearInterval(updateInterval)
  
  updateInterval = setInterval(() => {
    if (playbackState.value.isPlaying) {
      currentTime.value++
      if (currentTime.value >= playbackState.value.duration) {
        currentTime.value = playbackState.value.duration
        playbackState.value.isPlaying = false
        if (updateInterval) clearInterval(updateInterval)
      }
    }
  }, 1000)
}

// Seek to position
const onSeek = async () => {
  if (!playbackState.value.canSeek) return
  
  try {
    await SeekTo(
      playbackState.value.deviceUrl,
      playbackState.value.mediaPath,
      seekPosition.value
    )
    currentTime.value = seekPosition.value
  } catch (err) {
    console.error('Seek failed:', err)
  }
}

// Seek relative
const seekRelative = async (seconds: number) => {
  const newTime = Math.max(0, Math.min(playbackState.value.duration, currentTime.value + seconds))
  seekPosition.value = newTime
  await onSeek()
}

// Stop playback
const stopPlayback = async () => {
  try {
    await StopPlayback()
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

// Watch for playback state changes
watch(() => playbackState.value.isPlaying, (isPlaying) => {
  if (isPlaying) {
    startTimeUpdate()
  } else if (updateInterval) {
    clearInterval(updateInterval)
  }
})

onMounted(() => {
  loadPlaybackState()
  startTimeUpdate()
  
  // Poll for state updates every 2 seconds
  const pollInterval = setInterval(loadPlaybackState, 2000)
  
  onUnmounted(() => {
    if (updateInterval) clearInterval(updateInterval)
    clearInterval(pollInterval)
  })
})
</script>

<style scoped>
.playback-control {
  background: #2a2a2a;
  border-radius: 8px;
  padding: 20px;
  margin: 20px 0;
  color: #ffffff;
}

.playback-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
}

.playback-header h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}

.stop-btn {
  background: #e74c3c;
  color: white;
  border: none;
  padding: 8px 16px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
}

.stop-btn:hover {
  background: #c0392b;
}

.playback-info {
  display: flex;
  justify-content: space-between;
  margin-bottom: 15px;
  font-size: 14px;
  color: #aaa;
}

.device-name {
  font-weight: 500;
}

.time-display {
  font-family: monospace;
}

.seek-controls {
  margin-bottom: 15px;
}

.seek-container {
  position: relative;
  width: 100%;
}

.seek-tooltip {
  position: absolute;
  bottom: 25px;
  transform: translateX(-50%);
  background: rgba(0, 0, 0, 0.9);
  color: white;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-family: monospace;
  pointer-events: none;
  white-space: nowrap;
  z-index: 10;
}

.seek-tooltip::after {
  content: '';
  position: absolute;
  top: 100%;
  left: 50%;
  transform: translateX(-50%);
  border: 4px solid transparent;
  border-top-color: rgba(0, 0, 0, 0.9);
}

.seek-bar {
  width: 100%;
  height: 6px;
  border-radius: 3px;
  background: #444;
  outline: none;
  -webkit-appearance: none;
  cursor: pointer;
}

.seek-bar::-webkit-slider-thumb {
  -webkit-appearance: none;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: #3498db;
  cursor: pointer;
}

.seek-bar::-moz-range-thumb {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: #3498db;
  cursor: pointer;
  border: none;
}

.playback-controls {
  display: flex;
  gap: 10px;
  justify-content: center;
}

.control-btn {
  background: #3498db;
  color: white;
  border: none;
  padding: 8px 16px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
  font-weight: 500;
}

.control-btn:hover {
  background: #2980b9;
}

.control-btn:active {
  transform: scale(0.95);
}
</style>
