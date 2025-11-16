<script setup lang="ts">
import { ref } from "vue";
import { useCastStore } from "../stores/cast";
import { mediaService } from "../services/media";
import { fileService } from "../services/file";
import "./FileExplorer.css";
import "./common.css";

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

const openDefaultFolder = () => {
  // Just skip loading on mount - user will click dialog buttons
  return;
};

// Load default folder on mount
openDefaultFolder();
</script>

<template>
  <div class="file-explorer">
    <div class="explorer-header">
      <h2>🎥 Media Files</h2>
      <div class="path-input-group">
        <input
          v-model="manualPath"
          type="text"
          placeholder="Enter folder path..."
          class="path-input"
          @keyup.enter="handlePathChange"
        />
        <button @click="openFolderDialog" class="dialog-btn">📁 Folder</button>
        <button @click="openFileDialog" class="dialog-btn">📄 File</button>
      </div>
    </div>

    <div class="files-list">
      <div v-if="isLoading" class="loading-state">
        <div class="spinner"></div>
        <p>Loading media files...</p>
      </div>

      <div v-else-if="files.length > 0" class="files-container">
        <div
          v-for="file in files"
          :key="file"
          class="file-item"
          @click="selectMedia(file)"
        >
          <span class="file-icon">🎬</span>
          <span class="file-name">{{ file.split("/").pop() }}</span>
          <span class="file-path">{{ file }}</span>
          <span class="select-arrow">→</span>
        </div>
      </div>

      <div v-else class="empty-state">
        <div class="empty-icon">📁</div>
        <h3>No Media Files</h3>
        <p>Use the dialog buttons to select a file or folder.</p>
      </div>
    </div>
  </div>
</template>
