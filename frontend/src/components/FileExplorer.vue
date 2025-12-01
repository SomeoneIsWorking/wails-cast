<script setup lang="ts">
import { ref } from "vue";
import { useCastStore } from "../stores/cast";
import { Play } from "lucide-vue-next";
import TrackSelectionModal from "./TrackSelectionModal.vue";
import FileSelector from "./FileSelector.vue";
import History from "./History.vue";
import { GetTrackDisplayInfo } from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";
import LoadingIcon from './LoadingIcon.vue'
import { isAcceptedFileWithHttp } from "@/utils/file";
const emit = defineEmits<{
  select: [path: string];
}>();

const store = useCastStore();
const selectedFile = ref("");
const isLoading = ref(false);
const trackInfo = ref<main.TrackDisplayInfo | null>(null);
const showTrackModal = ref(false);

const handleCast = async (mediaPath: string) => {
  if (!store.selectedDevice) return;

  isLoading.value = true;
  try {
    trackInfo.value = await GetTrackDisplayInfo(mediaPath);
    showTrackModal.value = true;
  } finally {
    isLoading.value = false;
  }
};

const handleHistorySelect = (path: string) => {
  selectedFile.value = path;
};

const acceptedExtensions = [
  "mp4",
  "mkv",
  "avi",
  "mov",
  "wmv",
  "flv",
  "webm",
  "m3u8",
];
</script>

<template>
  <div class="file-explorer h-full flex flex-col">
    <div class="pm-2">
      <FileSelector
        v-model="selectedFile"
        :accepted-extensions="acceptedExtensions"
      >
        <button
          @click="handleCast(selectedFile)"
          :disabled="!selectedFile || isLoading || !store.selectedDevice || !isAcceptedFileWithHttp(selectedFile, acceptedExtensions)"
          class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors duration-200"
        >
          <Play v-if="!isLoading" class="h-4 w-4 mr-2" />
          <LoadingIcon v-else class="h-4 w-4 mr-2" />
          {{ isLoading ? "Loading..." : "Cast File" }}
        </button>
      </FileSelector>
    </div>

    <!-- History Section -->
    <div class="flex-1 overflow-hidden">
      <History @select="handleHistorySelect" />
    </div>

    <!-- Track Selection Modal -->
    <Suspense>
      <TrackSelectionModal v-model="showTrackModal" :track-info="trackInfo!" />
    </Suspense>
  </div>
</template>
