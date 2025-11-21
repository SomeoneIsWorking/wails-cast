<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import { Upload } from 'lucide-vue-next';
import { OnFileDrop } from '../../wailsjs/runtime/runtime';
import { OpenFileDialog } from '../../wailsjs/go/main/App';

interface Props {
  modelValue?: string;
  acceptedExtensions: string[];
  placeholder?: string;
  dialogTitle?: string;
  dialogFilters?: string[];
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: '',
  placeholder: 'Drag & drop a file or click Browse',
  dialogTitle: 'Select File'
});

const emit = defineEmits<{
  'update:modelValue': [path: string];
}>();

const dropZoneRef = ref<HTMLElement | null>(null);
const isHovering = ref(false);

const isAcceptedFile = (filePath: string): boolean => {
  const ext = filePath.toLowerCase().split('.').pop();
  return props.acceptedExtensions.some(accepted => 
    accepted.toLowerCase() === ext || accepted === '*'
  );
};

const handleFileSelect = (filePath: string) => {
  if (isAcceptedFile(filePath)) {
    emit("update:modelValue", filePath);
  }
};

const openFileDialog = async () => {
  try {
    const filters = props.dialogFilters || props.acceptedExtensions.map(ext => `*.${ext}`);
    const file = await OpenFileDialog(props.dialogTitle, filters);
    if (file) {
      handleFileSelect(file);
    }
  } catch (error) {
    console.error("Failed to open file dialog", error);
  }
};

let dropHandler: ((x: number, y: number, paths: string[]) => void) | null = null;

// Setup Wails file drop handler
onMounted(() => {
  dropHandler = (_x: number, _y: number, paths: string[]) => {
    // Only handle drops on this specific component's drop zone
    if (!dropZoneRef.value || !isHovering.value) {
      return;
    }
    
    if (paths && paths.length > 0) {
      const filePath = paths[0];
      if (isAcceptedFile(filePath)) {
        handleFileSelect(filePath);
      }
    }
    isHovering.value = false;
  };
  
  OnFileDrop(dropHandler, true);
});

onUnmounted(() => {
  // Clean up handler
  dropHandler = null;
});
</script>

<template>
  <div class="file-selector">
    <div class="flex gap-2 mb-4">
      <input
        :value="modelValue"
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
      ref="dropZoneRef"
      class="drop-zone flex space-x-2"
      style="--wails-drop-target: drop"
      @click="openFileDialog"
      @mouseenter="isHovering = true"
      @mouseleave="isHovering = false"
    >
      <Upload :size="48" class="text-gray-500 mb-3 mr-5" />
      <div>
        <p class="text-lg font-medium text-gray-300 mb-1">
          Drag & drop a file
        </p>
        <p class="text-sm text-gray-500">
          or click to browse
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.file-selector {
  width: 100%;
}
</style>
