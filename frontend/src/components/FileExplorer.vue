<script setup lang="ts">
import { ref } from "vue";
import { useCastStore } from "../stores/cast";
import { ArrowRightCircle } from "lucide-vue-next";
import FileSelector from "./FileSelector.vue";
import History from "./History.vue";
import { GetTrackDisplayInfo } from "../../wailsjs/go/main/App";
import LoadingIcon from "./LoadingIcon.vue";
import { isAcceptedFileWithHttp } from "@/utils/file";
const emit = defineEmits<{
  options: [];
}>();

const store = useCastStore();
const selectedFile = ref("");
const isLoading = ref(false);

const handleCast = async (mediaPath: string) => {
  if (!store.selectedDevice) return;

  isLoading.value = true;
  try {
    const trackInfo = await GetTrackDisplayInfo(mediaPath);
    store.setTrackInfo(trackInfo);
    emit("options");
  } finally {
    isLoading.value = false;
  }
};

const handleHistorySelect = async (path: string) => {
  await handleCast(path);
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
          :disabled="
            !selectedFile ||
            isLoading ||
            !store.selectedDevice ||
            !isAcceptedFileWithHttp(selectedFile, acceptedExtensions)
          "
          class="btn-primary"
        >
          <ArrowRightCircle v-if="!isLoading" class="h-4 w-4 mr-2" />
          <LoadingIcon v-else class="h-4 w-4 mr-2" />
          {{ isLoading ? "Loading..." : "Next" }}
        </button>
      </FileSelector>
    </div>

    <!-- History Section -->
    <div class="flex-1 overflow-hidden">
      <History @select="handleHistorySelect" />
    </div>
  </div>
</template>
