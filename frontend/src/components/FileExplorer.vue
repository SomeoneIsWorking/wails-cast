<script setup lang="ts">
import { ref, watch } from "vue";
import { useCastStore } from "../stores/cast";
import FileSelector from "./FileSelector.vue";
import { remoteMediaService } from "../services/remoteMedia";
import { Link, Play } from "lucide-vue-next";

const emit = defineEmits<{
  select: [path: string];
}>();

const store = useCastStore();
const selectedFile = ref("");
const remoteUrl = ref("");
const isCastingRemote = ref(false);

const selectMedia = (filePath: string) => {
  store.selectMedia(filePath);
  emit("select", filePath);
};

const castRemoteUrl = async () => {
  if (!remoteUrl.value || !store.selectedDevice) return;
  
  isCastingRemote.value = true;
  try {
    const deviceUrl = `${store.selectedDevice.host}:${store.selectedDevice.port}`;
    await remoteMediaService.castRemoteURL(remoteUrl.value, deviceUrl);
    // Clear error if any
    store.clearError();
  } catch (err) {
    console.error("Failed to cast remote URL", err);
    store.setError("Failed to cast remote URL: " + err);
  } finally {
    isCastingRemote.value = false;
  }
};

// Watch for file selection changes
watch(selectedFile, (newFile) => {
  if (newFile) {
    selectMedia(newFile);
  }
});
</script>

<template>
  <div class="file-explorer h-full flex flex-col">
    <div class="flex-1 overflow-auto space-y-6">
      <!-- File Selector -->
      <div>
        <h3 class="text-lg font-medium text-gray-300 mb-3">Local File</h3>
        <FileSelector
          v-model="selectedFile"
          :accepted-extensions="[
            'mp4',
            'mkv',
            'webm',
            'avi',
            'mov',
            'flv',
            'm4v',
          ]"
          placeholder="Drag & drop a video file or click Browse"
          dialog-title="Select Video File"
        />
      </div>

      <!-- Divider -->
      <div class="relative">
        <div class="absolute inset-0 flex items-center">
          <div class="w-full border-t border-gray-700"></div>
        </div>
        <div class="relative flex justify-center text-sm">
          <span class="px-2 bg-gray-900 text-gray-500">OR</span>
        </div>
      </div>

      <!-- Remote URL Input -->
      <div>
        <h3 class="text-lg font-medium text-gray-300 mb-3">Remote URL</h3>
        <div class="bg-gray-800 rounded-lg border border-gray-700 p-4">
          <div class="flex gap-2">
            <div class="relative flex-1">
              <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                <Link class="h-5 w-5 text-gray-400" />
              </div>
              <input
                v-model="remoteUrl"
                type="text"
                class="block w-full pl-10 pr-3 py-2 border border-gray-600 rounded-md leading-5 bg-gray-700 text-gray-300 placeholder-gray-400 focus:outline-none focus:bg-gray-600 focus:border-blue-500 sm:text-sm transition duration-150 ease-in-out"
                placeholder="https://example.com/video.mp4"
                @keyup.enter="castRemoteUrl"
              />
            </div>
            <button
              @click="castRemoteUrl"
              :disabled="!remoteUrl || isCastingRemote || !store.selectedDevice"
              class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors duration-200"
            >
              <Play v-if="!isCastingRemote" class="h-4 w-4 mr-2" />
              <svg
                v-else
                class="animate-spin -ml-1 mr-2 h-4 w-4 text-white"
                xmlns="http://www.w3.org/2000/svg"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                ></circle>
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                ></path>
              </svg>
              {{ isCastingRemote ? "Casting..." : "Cast URL" }}
            </button>
          </div>
          <p class="mt-2 text-sm text-gray-500">
            Enter a direct link to a video file or HLS stream (.m3u8)
          </p>
        </div>
      </div>
    </div>
  </div>
</template>
