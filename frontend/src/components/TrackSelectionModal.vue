<script setup lang="ts">
import { ref, computed } from 'vue'
import type { main, mediainfo } from '../../wailsjs/go/models'

type TrackInfo = mediainfo.MediaTrackInfo;

const props = defineProps<{
  trackInfo: TrackInfo
  modelValue: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'confirm': [options: main.CastOptions]
}>()

const selectedVideoTrack = ref(-1)
const selectedAudioTrack = ref(-1)
const selectedSubtitleTrack = ref(-1)
const subtitleSource = ref<'none' | 'embedded' | 'external'>('none')
const externalSubtitlePath = ref('')
const burnSubtitles = ref(false)
const quality = ref<'low' | 'medium' | 'high' | 'original'>('medium')

const hasMultipleVideoTracks = computed(() => props.trackInfo.videoTracks.length > 1)
const hasMultipleAudioTracks = computed(() => props.trackInfo.audioTracks.length > 1)
const hasSubtitleTracks = computed(() => props.trackInfo.subtitleTracks.length > 0)

const showDialog = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const handleConfirm = () => {
  const options: main.CastOptions = {
    SubtitlePath: subtitleSource.value === 'external' ? externalSubtitlePath.value : '',
    SubtitleTrack: subtitleSource.value === 'embedded' ? selectedSubtitleTrack.value : -1,
    VideoTrack: selectedVideoTrack.value,
    AudioTrack: selectedAudioTrack.value,
    BurnIn: burnSubtitles.value,
    Quality: quality.value
  }
  emit('confirm', options)
  showDialog.value = false
}

const handleCancel = () => {
  showDialog.value = false
}
</script>

<template>
  <div v-if="showDialog" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" @click.self="handleCancel">
    <div class="bg-gray-800 rounded-lg p-6 max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
      <h2 class="text-2xl font-bold text-white mb-4">Select Tracks</h2>
      
      <!-- Video Track Selection -->
      <div v-if="hasMultipleVideoTracks" class="mb-6">
        <h3 class="text-lg font-semibold text-white mb-2">Video Track</h3>
        <select v-model="selectedVideoTrack" class="w-full bg-gray-700 text-white rounded p-2">
          <option :value="-1">Default</option>
          <option v-for="track in trackInfo.videoTracks" :key="track.index" :value="track.index">
            Track {{ track.index }} - {{ track.codec }} {{ track.resolution || '' }}
          </option>
        </select>
      </div>

      <!-- Audio Track Selection -->
      <div v-if="hasMultipleAudioTracks" class="mb-6">
        <h3 class="text-lg font-semibold text-white mb-2">Audio Track</h3>
        <select v-model="selectedAudioTrack" class="w-full bg-gray-700 text-white rounded p-2">
          <option :value="-1">Default</option>
          <option v-for="track in trackInfo.audioTracks" :key="track.index" :value="track.index">
            Track {{ track.index }} - {{ track.language || 'Unknown' }} ({{ track.codec }})
          </option>
        </select>
      </div>

      <!-- Subtitle Selection -->
      <div class="mb-6">
        <h3 class="text-lg font-semibold text-white mb-2">Subtitles</h3>
        <select v-model="subtitleSource" class="w-full bg-gray-700 text-white rounded p-2 mb-2">
          <option value="none">No Subtitles</option>
          <option v-if="hasSubtitleTracks" value="embedded">Embedded Subtitle</option>
          <option value="external">External File</option>
        </select>

        <select v-if="subtitleSource === 'embedded'" v-model="selectedSubtitleTrack" class="w-full bg-gray-700 text-white rounded p-2 mb-2">
          <option v-for="track in trackInfo.subtitleTracks" :key="track.index" :value="track.index">
            {{ track.title || track.language || `Track ${track.index}` }} ({{ track.codec }})
          </option>
        </select>

        <div v-if="subtitleSource !== 'none'" class="mt-2">
          <label class="flex items-center text-white">
            <input type="checkbox" v-model="burnSubtitles" class="mr-2">
            Burn subtitles into video
          </label>
        </div>
      </div>

      <!-- Quality Selection -->
      <div class="mb-6">
        <h3 class="text-lg font-semibold text-white mb-2">Quality</h3>
        <select v-model="quality" class="w-full bg-gray-700 text-white rounded p-2">
          <option value="original">Original (Best Quality)</option>
          <option value="high">High (CRF 23)</option>
          <option value="medium">Medium (CRF 28)</option>
          <option value="low">Low (CRF 35)</option>
        </select>
      </div>

      <!-- Actions -->
      <div class="flex gap-3 justify-end">
        <button @click="handleCancel" class="px-4 py-2 bg-gray-600 text-white rounded hover:bg-gray-700 transition">
          Cancel
        </button>
        <button @click="handleConfirm" class="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 transition">
          Start Casting
        </button>
      </div>
    </div>
  </div>
</template>
