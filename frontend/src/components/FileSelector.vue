<script setup lang="ts">
import { ref } from "vue";
import { Upload } from "lucide-vue-next";
// Central Wails drop events are dispatched as 'wails-file-drop' from main.ts
import useOnFileDrop from "../hooks/useOnFileDrop";
import { isAcceptedFile } from "../utils/file";
import { OpenFileDialog } from "../../wailsjs/go/main/App";

interface Props {
  acceptedExtensions: string[];
  placeholder?: string;
  dialogTitle?: string;
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: "",
  dialogTitle: "Select File",
});

const modelValue = defineModel<string>();

const dropZoneRef = ref<HTMLElement | null>(null);
const isHovering = ref(false);


const handleFileSelect = (filePath: string) => {
  if (isAcceptedFile(filePath, props.acceptedExtensions)) {
    modelValue.value = filePath;
  }
};

const openFileDialog = async () => {
  const filters = props.acceptedExtensions
    .filter((ext) => ext && ext !== "*")
    .map((ext) => `*.${ext}`);
  const file = await OpenFileDialog(props.dialogTitle, filters);
  if (file) {
    handleFileSelect(file);
  }
};

// useOnFileDrop will manage the global listener

// Setup Wails file drop handler
useOnFileDrop({
  dropZoneRef,
  acceptedExtensions: props.acceptedExtensions,
  onDrop: (paths) => {
    if (paths && paths.length > 0) {
      const filePath = paths[0];
      handleFileSelect(filePath);
    }
    isHovering.value = false;
  },
});
</script>

<template>
  <div class="file-selector">
    <div class="flex gap-2 mb-4">
      <input
        v-model="modelValue"
        type="text"
        placeholder="Input URL or file path"
        class="input-field flex-1"
      />
      <button @click="openFileDialog" class="btn-secondary">Browse</button>
      <slot></slot>
    </div>

    <!-- Drag and Drop Zone -->
    <div
      ref="dropZoneRef"
      class="drop-zone flex space-x-2"
      style="--wails-drop-target: drop"
      @click="openFileDialog"
      @mouseenter="isHovering = true"
      @mouseleave="isHovering = false"
    >
      <Upload :size="48" class="text-gray-500 mb-3 mr-5" />
      <div>
        <p class="text-lg font-medium text-gray-300 mb-1">Drag & drop a file</p>
        <p class="text-sm text-gray-500">or click to browse</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.file-selector {
  width: 100%;
}
</style>
