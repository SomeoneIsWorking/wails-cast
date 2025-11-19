<script setup lang="ts">
import { ref, computed, onMounted, watch } from "vue";
import { useCastStore } from "../stores/cast";
import { mediaService } from "../services/media";
import { FindSubtitleFile, GetSubtitleTracks } from "../../wailsjs/go/main/App";
import type { main } from "../../wailsjs/go/models";
import type { Device } from "../stores/cast";
import { ArrowLeft, Cast, Video, Loader2, Check, Languages } from 'lucide-vue-next';
import FileSelector from "./FileSelector.vue";

interface Props {
  device: Device;
  mediaPath: string;
  isLoading: boolean;
}

defineProps<Props>();

const emit = defineEmits<{
  cast: [];
  back: [];
}>();

const store = useCastStore();
const isCasting = ref(false);
const castResult = ref<string | null>(null);
const mediaURL = ref<string>("");
const subtitlePath = ref<string>("");
const subtitleTracks = ref<main.SubtitleTrack[]>([]);
const selectedSubtitleSource = ref<string>("none"); // "none", "external", or track index as string
const autoCastDone = ref(false);
const fileSelectorRef = ref<InstanceType<typeof FileSelector>>();

const fileName = computed(() => store.selectedMedia?.split("/").pop() || "");

// Auto-detect subtitle file and auto-cast on mount
onMounted(async () => {
  if (store.selectedMedia) {
    try {
      // Load subtitle tracks from video file
      const tracks = await GetSubtitleTracks(store.selectedMedia);
      subtitleTracks.value = tracks;
      
      // If tracks exist, select the first one by default
      if (tracks.length > 0) {
        selectedSubtitleSource.value = tracks[0].index.toString();
      } else {
        // Try to find external subtitle file
        const foundSub = await FindSubtitleFile(store.selectedMedia);
        if (foundSub) {
          subtitlePath.value = foundSub;
          selectedSubtitleSource.value = "external";
        }
      }
    } catch (err) {
      console.error("Failed to load subtitles:", err);
    }
  }
  await generateMediaURL();
  
  // Auto-cast if not already done
  if (!autoCastDone.value) {
    autoCastDone.value = true;
    await handleCast();
  }
});

const handleSubtitleFileSelect = async (path: string) => {
  subtitlePath.value = path;
  await applySubtitleSettings();
};

// Watch for subtitle source changes and apply immediately
watch(selectedSubtitleSource, async () => {
  if (autoCastDone.value) {
    await applySubtitleSettings();
  }
});

const applySubtitleSettings = async () => {
  if (!store.selectedDevice || !store.selectedMedia) return;

  try {
    let finalSubtitlePath = "";
    let subtitleTrack = -1;

    if (selectedSubtitleSource.value === "external") {
      finalSubtitlePath = subtitlePath.value;
    } else if (selectedSubtitleSource.value !== "none") {
      subtitleTrack = parseInt(selectedSubtitleSource.value);
    }

    // Update subtitle settings on the server (backend handles cache clearing and seek)
    await mediaService.updateSubtitleSettings(finalSubtitlePath, subtitleTrack);
  } catch (error: unknown) {
    console.error("Failed to update subtitle settings:", error);
    store.setError(error instanceof Error ? error.message : String(error));
  }
};

const recast = async () => {
  await handleCast(false);
};

const handleCast = async (emitCastEvent = true) => {
  isCasting.value = true;
  castResult.value = null;

  try {
    let finalSubtitlePath = "";
    let subtitleTrack = -1;

    if (selectedSubtitleSource.value === "external") {
      finalSubtitlePath = subtitlePath.value;
    } else if (selectedSubtitleSource.value !== "none") {
      subtitleTrack = parseInt(selectedSubtitleSource.value);
    }

    await mediaService.castToDevice(
      store.selectedDevice!.url,
      store.selectedMedia!,
      finalSubtitlePath,
      subtitleTrack
    );
    castResult.value = "Casting to " + store.selectedDevice!.name;
    store.clearError();
    if (emitCastEvent) {
      emit('cast');
    }
  } catch (error: unknown) {
    const errorMsg = error instanceof Error ? error.message : String(error);
    store.setError(errorMsg);
    castResult.value = null;
  } finally {
    isCasting.value = false;
  }
};

const generateMediaURL = async () => {
  try {
    const url = await mediaService.getMediaURL(store.selectedMedia!);
    mediaURL.value = url;
  } catch (error: unknown) {
    store.setError("Failed to generate media URL");
  }
};

const copyToClipboard = () => {
  navigator.clipboard.writeText(mediaURL.value);
};
</script>

<template>
  <div class="card">
    <div class="card-header">
      <div class="flex items-center justify-between">
        <button @click="$emit('back')" class="btn-secondary flex items-center gap-2">
          <ArrowLeft :size="18" />
          Back
        </button>
        <h2 class="text-2xl font-bold flex items-center gap-2">
          <Cast :size="28" class="text-green-400" />
          Cast Media
        </h2>
        <div class="w-20"></div>
      </div>
    </div>

    <div class="card-body space-y-6">
      <!-- Casting Status -->
      <div v-if="isCasting || isLoading" class="flex flex-col items-center justify-center py-8 bg-blue-900/20 rounded-lg border border-blue-700">
        <Loader2 :size="56" class="text-blue-400 mb-4 animate-spin" />
        <p class="text-lg font-medium text-blue-400">Starting playback...</p>
        <p class="text-sm text-gray-400 mt-1">Initializing stream</p>
      </div>

      <div v-else-if="castResult" class="flex items-center gap-3 p-4 bg-green-900/30 border border-green-700 rounded-lg">
        <Check :size="24" class="text-green-400" />
        <div>
          <p class="font-medium text-green-400">{{ castResult }}</p>
          <p class="text-sm text-gray-400">Use the playback controls below to control playback</p>
        </div>
      </div>

      <!-- Media Info -->
      <div class="flex items-center gap-4 p-4 bg-gray-700 rounded-lg">
        <div class="p-3 bg-purple-600 rounded-lg">
          <Video :size="32" />
        </div>
        <div class="flex-1 min-w-0">
          <h3 class="font-semibold text-lg truncate">{{ fileName }}</h3>
          <p class="text-sm text-gray-400 truncate">{{ mediaPath }}</p>
        </div>
      </div>

      <!-- Device Info -->
      <div class="flex items-center gap-4 p-4 bg-gray-700 rounded-lg">
        <div class="p-3 bg-blue-600 rounded-lg">
          <Cast :size="32" />
        </div>
        <div class="flex-1 min-w-0">
          <h3 class="font-semibold text-lg truncate">{{ device.name }}</h3>
          <p class="text-sm text-gray-400">{{ device.type }}</p>
          <p class="text-xs text-gray-500">{{ device.address }}</p>
        </div>
      </div>

      <!-- Subtitles Section -->
      <div class="space-y-3">
        <label class="flex items-center gap-2 text-sm font-medium text-gray-300">
          <Languages :size="20" />
          Subtitles
        </label>
        
        <select 
          v-model="selectedSubtitleSource" 
          class="select-field"
        >
          <option value="none">No Subtitles</option>
          <option 
            v-for="track in subtitleTracks" 
            :key="track.index" 
            :value="track.index.toString()"
          >
            Track {{ track.index }}
            <template v-if="track.language"> ({{ track.language }})</template>
            <template v-if="track.title"> - {{ track.title }}</template>
          </option>
          <option value="external">External File...</option>
        </select>

        <div v-if="selectedSubtitleSource === 'external'" class="mt-3">
          <FileSelector
            ref="fileSelectorRef"
            :accepted-extensions="['srt', 'vtt', 'ass', 'ssa']"
            placeholder="Select subtitle file"
            dialog-title="Select Subtitle File"
            @select="handleSubtitleFileSelect"
          />
        </div>

        <p v-if="selectedSubtitleSource !== 'none'" class="text-xs text-green-400 flex items-center gap-1">
          <Check :size="14" />
          Subtitles will be burned into video
        </p>
      </div>

      <!-- Media URL Section -->
      <div v-if="mediaURL" class="space-y-2">
        <label class="text-sm font-medium text-gray-300">Stream URL</label>
        <div class="flex gap-2">
          <input 
            :value="mediaURL" 
            readonly 
            class="input-field flex-1 font-mono text-sm"
          />
          <button @click="copyToClipboard" class="btn-secondary flex items-center gap-2">
            <Check :size="18" />
            Copy
          </button>
        </div>
      </div>

      <!-- Recast Button -->
      <div class="flex justify-end gap-3 pt-4">
        <button @click="$emit('back')" class="btn-secondary">
          Cancel
        </button>
        <button
          @click="recast"
          :disabled="isCasting || isLoading"
          class="btn-success flex items-center gap-2"
        >
          <Loader2 v-if="isCasting || isLoading" :size="18" class="animate-spin" />
          <Cast v-else :size="18" />
          {{ isCasting || isLoading ? "Casting..." : "Recast" }}
        </button>
      </div>
    </div>
  </div>
</template>
