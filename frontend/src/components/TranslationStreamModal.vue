<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import { X } from "lucide-vue-next";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";

defineProps<{
  targetLanguage: string;
}>();

const showDialog = defineModel<boolean>();
const streamContent = ref("");

onMounted(() => {
  EventsOn("translation:stream", (chunk: string) => {
    streamContent.value += chunk;
  });
});

onUnmounted(() => {
  EventsOff("translation:stream");
});

const handleClose = () => {
  showDialog.value = false;
  streamContent.value = "";
};
</script>

<template>
  <div
    v-if="showDialog"
    class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
    @click.self="handleClose"
  >
    <div
      class="bg-gray-800 rounded-lg p-6 max-w-4xl w-full mx-4 max-h-[80vh] flex flex-col"
    >
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-2xl font-bold text-white">
          Translating to {{ targetLanguage }}
        </h2>
        <button
          @click="handleClose"
          class="text-gray-400 hover:text-white transition"
        >
          <X class="w-6 h-6" />
        </button>
      </div>

      <div
        class="flex-1 overflow-y-auto bg-gray-900 rounded p-4 font-mono text-sm text-green-400"
      >
        <pre class="whitespace-pre-wrap text-left">{{ streamContent }}</pre>
      </div>
    </div>
  </div>
</template>
