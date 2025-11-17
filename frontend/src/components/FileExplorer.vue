<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useCastStore } from "../stores/cast";
import { mediaService } from "../services/media";
import { fileService } from "../services/file";
import { Video, Folder, File, Upload, Loader2, Check } from 'lucide-vue-next';
import { OnFileDrop } from '../../wailsjs/runtime/runtime';

const emit = defineEmits<{
  select: [path: string];
  loading: [isLoading: boolean];
}>();

const store = useCastStore();
const currentPath = ref("/");
const files = ref<string[]>([]);
const isLoading = ref(false);
const manualPath = ref("");

const loadMediaFiles = async (path: string) => {
  isLoading.value = true;
  emit("loading", true);

  try {
    const mediaFiles = await mediaService.getMediaFiles(path);
    files.value = mediaFiles.filter((f) => mediaService.isMediaFile(f)).sort();
    currentPath.value = path;
    manualPath.value = "";
  } catch (error: unknown) {
    store.setError("Failed to load files");
    files.value = [];
  } finally {
    isLoading.value = false;
    emit("loading", false);
  }
};

const selectMedia = (filePath: string) => {
  store.selectMedia(filePath);
  emit("select", filePath);
};

const handlePathChange = () => {
  if (manualPath.value.trim()) {
    loadMediaFiles(manualPath.value);
  }
};

const openFileDialog = async () => {
  try {
    const file = await fileService.selectMediaFile();
    if (file) {
      selectMedia(file);
    }
  } catch (error: unknown) {
    store.setError("Failed to open file dialog");
  }
};

const openFolderDialog = async () => {
  try {
    const folder = await fileService.selectMediaFolder();
    if (folder) {
      loadMediaFiles(folder);
    }
  } catch (error: unknown) {
    store.setError("Failed to open folder dialog");
  }
};

// Setup Wails file drop handler
onMounted(() => {
  // Use Wails OnFileDrop API to get actual file paths
  // useDropTarget: true enables the wails-drop-target-active class on elements with --wails-drop-target CSS property
  OnFileDrop((_x: number, _y: number, paths: string[]) => {
    if (paths && paths.length > 0) {
      const filePath = paths[0];
      if (mediaService.isMediaFile(filePath)) {
        selectMedia(filePath);
      } else {
        store.setError("Unsupported file type. Please drop a video file.");
      }
    }
  }, true);
});

</script>

<template>
  <div class="card">
    <div class="card-header">
      <h2 class="text-2xl font-bold flex items-center gap-2">
        <Video :size="28" class="text-purple-400" />
        Select Media
      </h2>
    </div>

    <div class="card-body">
      <!-- Path Input and Dialogs -->
      <div class="flex gap-2 mb-6">
        <input
          v-model="manualPath"
          type="text"
          placeholder="Enter folder path..."
          class="input-field flex-1"
          @keyup.enter="handlePathChange"
        />
        <button @click="openFolderDialog" class="btn-secondary flex items-center gap-2">
          <Folder :size="18" />
          Folder
        </button>
        <button @click="openFileDialog" class="btn-secondary flex items-center gap-2">
          <File :size="18" />
          File
        </button>
      </div>

      <!-- Drag and Drop Zone -->
      <div
        class="drop-zone"
        style="--wails-drop-target: drop"
      >
        <Upload :size="48" class="text-gray-500 mb-3" />
        <p class="text-lg font-medium text-gray-300 mb-1">
          Drag & drop a video file
        </p>
        <p class="text-sm text-gray-500">
          or use the buttons above to browse
        </p>
      </div>

      <!-- Loading State -->
      <div v-if="isLoading" class="flex flex-col items-center justify-center py-12 mt-6">
        <Loader2 :size="48" class="text-purple-400 mb-4 animate-spin" />
        <p class="text-gray-400">Loading media files...</p>
      </div>

      <!-- Files List -->
      <div v-else-if="files.length > 0" class="mt-6">
        <p class="text-sm text-gray-400 mb-3">
          {{ files.length }} media file{{ files.length > 1 ? 's' : '' }} found
        </p>
        <div class="space-y-2 max-h-96 overflow-y-auto">
          <div
            v-for="file in files"
            :key="file"
            @click="selectMedia(file)"
            :class="['file-item', {
              'file-item-selected': store.selectedMedia === file
            }]"
          >
            <Video :size="20" class="text-purple-400" />
            <div class="flex-1 min-w-0">
              <p class="font-medium truncate">{{ file.split("/").pop() }}</p>
              <p class="text-xs text-gray-500 truncate">{{ file }}</p>
            </div>
            <Check :size="20" v-if="store.selectedMedia === file" class="text-blue-400" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
