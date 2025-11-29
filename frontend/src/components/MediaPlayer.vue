<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useCastStore } from "../stores/cast";
import { mediaService } from "../services/media";
import {
  FindSubtitleFile,
  GetSubtitleTracks,
  ClearCache,
} from "../../wailsjs/go/main/App";
import type { mediainfo } from "../../wailsjs/go/models";
import {
  ArrowLeft,
  Cast,
  Video,
  Loader2,
  Check,
  Languages,
  Trash2,
} from "lucide-vue-next";
import FileSelector from "./FileSelector.vue";
import { Device } from "@/services/device";

interface Props {
  device: Device;
  mediaPath: string;
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
const subtitleTracks = ref<mediainfo.SubtitleTrack[]>([]);
const selectedSubtitleSource = ref<string>("none"); // "none", "external", or track index as string

const fileName = computed(() => store.selectedMedia?.split("/").pop() || "");

// Auto-detect subtitle file and auto-cast on mount
onMounted(async () => {
  if (store.selectedMedia) {
    try {
      // First priority: Try to find external subtitle file (e.g., .srt next to video)
      const foundSub = await FindSubtitleFile(store.selectedMedia);
      if (foundSub) {
        subtitlePath.value = foundSub;
        selectedSubtitleSource.value = "external";
      }

      // Load subtitle tracks from video file
      const tracks = await GetSubtitleTracks(store.selectedMedia);
      subtitleTracks.value = tracks;

      // If no external subtitle found and embedded tracks exist, select the first one
      if (!foundSub && tracks.length > 0) {
        selectedSubtitleSource.value = tracks[0].index.toString();
      }
    } catch (err) {
      console.error("Failed to load subtitles:", err);
    }
  }
  await generateMediaURL();
});

const applySubtitleSettings = async () => {
  if (!store.selectedDevice || !store.selectedMedia) return;

  let finalSubtitlePath = "";
  let subtitleTrack = -1;

  if (selectedSubtitleSource.value === "external") {
    finalSubtitlePath = subtitlePath.value;
  } else if (selectedSubtitleSource.value !== "none") {
    subtitleTrack = parseInt(selectedSubtitleSource.value);
  }

  // Update subtitle settings on the server (backend handles cache clearing and seek)
  await mediaService.updateSubtitleSettings({
    SubtitlePath: finalSubtitlePath,
    SubtitleTrack: subtitleTrack,
    BurnIn: true
  });
};

const recast = async () => {
  isCasting.value = true;
  castResult.value = null;

  try {
    await mediaService.castToDevice(
      store.selectedDevice!.host,
      store.selectedMedia!,
      store.castOptions
    );
    castResult.value = "Casting to " + store.selectedDevice!.name;
    store.clearError();
  } finally {
    isCasting.value = false;
  }
};

const generateMediaURL = async () => {
    mediaURL.value = await mediaService.getMediaURL(store.selectedMedia!);
};

const clearCache = async () => {
    await ClearCache();
};
</script>

<template>
  <div class="media-player h-full flex flex-col">
    <div class="flex items-center justify-between mb-4">
      <button
        @click="$emit('back')"
        class="btn-secondary flex items-center gap-2"
      >
        <ArrowLeft :size="18" />
        Back
      </button>
      <div></div>
      <div class="w-20"></div>
    </div>
    <div class="flex-1 overflow-auto space-y-6">
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
        <label
          class="flex items-center gap-2 text-sm font-medium text-gray-300"
        >
          <Languages :size="20" />
          Subtitles
        </label>

        <select v-model="selectedSubtitleSource" class="select-field">
          <option value="none">No Subtitles</option>
          <option value="external">External File...</option>
          <option
            v-for="track in subtitleTracks"
            :key="track.index"
            :value="track.index.toString()"
          >
            Embedded Track {{ track.index }}
            <template v-if="track.language"> ({{ track.language }})</template>
            <template v-if="track.title"> - {{ track.title }}</template>
          </option>
        </select>

        <div v-if="selectedSubtitleSource === 'external'" class="mt-3">
          <FileSelector
            v-model="subtitlePath"
            :accepted-extensions="['srt', 'vtt', 'ass', 'ssa']"
            placeholder="Select subtitle file"
            dialog-title="Select Subtitle File"
          >
          <button @click="applySubtitleSettings" class="btn-primary">Apply</button>
        </FileSelector>
        </div>

        <p
          v-if="selectedSubtitleSource !== 'none'"
          class="text-xs text-green-400 flex items-center gap-1"
        >
          <Check :size="14" />
          Subtitles will be burned into video
        </p>
      </div>
    </div>
    <!-- Recast Button -->
    <div class="flex justify-between gap-3 pt-4">
      <button @click="clearCache" class="btn-secondary flex items-center gap-2">
        <Trash2 :size="18" />
        Clear Cache
      </button>
      <div class="flex gap-3">
        <button @click="$emit('back')" class="btn-secondary">Cancel</button>
        <button
          @click="recast"
          :disabled="isCasting"
          class="btn-success flex items-center gap-2"
        >
          <Loader2
            v-if="isCasting"
            :size="18"
            class="animate-spin"
          />
          <Cast v-else :size="18" />
          {{ isCasting ? "Casting..." : "Recast" }}
        </button>
      </div>
    </div>
  </div>
</template>
