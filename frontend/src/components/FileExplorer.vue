<script setup lang="ts">
import { ref, watch } from "vue";
import { useCastStore } from "../stores/cast";
import { Video } from 'lucide-vue-next';
import FileSelector from "./FileSelector.vue";

const emit = defineEmits<{
  select: [path: string];
}>();

const store = useCastStore();
const selectedFile = ref("");

const selectMedia = (filePath: string) => {
  store.selectMedia(filePath);
  emit("select", filePath);
};

// Watch for file selection changes
watch(selectedFile, (newFile) => {
  if (newFile) {
    selectMedia(newFile);
  }
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
      <!-- File Selector -->
      <FileSelector
        v-model="selectedFile"
        :accepted-extensions="['mp4', 'mkv', 'webm', 'avi', 'mov', 'flv', 'm4v', '*']"
        :dialog-filters="['*.mp4', '*.mkv', '*.webm', '*.avi', '*.mov', '*.flv', '*.m4v']"
        placeholder="Drag & drop a video file or click Browse"
        dialog-title="Select Video File"
      />
    </div>
  </div>
</template>

