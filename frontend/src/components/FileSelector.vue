<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Upload } from 'lucide-vue-next';
import { OnFileDrop } from '../../wailsjs/runtime/runtime';

interface Props {
  acceptedExtensions: string[];
  placeholder?: string;
  dialogTitle?: string;
  dialogFilter?: string;
}

const props = withDefaults(defineProps<Props>(), {
  placeholder: 'Drag & drop a file or click Browse',
  dialogTitle: 'Select File',
  dialogFilter: '*.*'
});

const emit = defineEmits<{
  select: [path: string];
}>();

const selectedPath = ref<string>("");

const isAcceptedFile = (filePath: string): boolean => {
  const ext = filePath.toLowerCase().split('.').pop();
  return props.acceptedExtensions.some(accepted => 
    accepted.toLowerCase() === ext || accepted === '*'
  );
};

const handleFileSelect = (filePath: string) => {
  if (isAcceptedFile(filePath)) {
    selectedPath.value = filePath;
    emit("select", filePath);
  }
};

const openFileDialog = async () => {
  try {
    const { OpenFileDialog } = await import('../../wailsjs/go/main/App');
    const file = await OpenFileDialog();
    if (file) {
      handleFileSelect(file);
    }
  } catch (error) {
    console.error("Failed to open file dialog", error);
  }
};

// Setup Wails file drop handler
onMounted(() => {
  OnFileDrop((_x: number, _y: number, paths: string[]) => {
    if (paths && paths.length > 0) {
      const filePath = paths[0];
      if (isAcceptedFile(filePath)) {
        handleFileSelect(filePath);
      }
    }
  }, true);
});

defineExpose({
  selectedPath,
  clear: () => { selectedPath.value = ""; }
});
</script>

<template>
  <div class="file-selector">
    <div class="flex gap-2 mb-4">
      <input
        v-model="selectedPath"
        type="text"
        :placeholder="placeholder"
        class="input-field flex-1"
        readonly
      />
      <button @click="openFileDialog" class="btn-secondary">
        Browse
      </button>
    </div>

    <!-- Drag and Drop Zone -->
    <div
      class="drop-zone"
      style="--wails-drop-target: drop"
      @click="openFileDialog"
    >
      <Upload :size="48" class="text-gray-500 mb-3" />
      <p class="text-lg font-medium text-gray-300 mb-1">
        Drag & drop a file
      </p>
      <p class="text-sm text-gray-500">
        or click to browse
      </p>
    </div>
  </div>
</template>

<style scoped>
.file-selector {
  width: 100%;
}
</style>
