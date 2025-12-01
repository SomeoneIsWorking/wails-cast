<script setup lang="ts">
import { ref, computed } from "vue";
import { useCastStore } from "../stores/cast";
import { mediaService } from "../services/media";
import {
  ClearCache,
} from "../../wailsjs/go/main/App";
import {
  ArrowLeft,
  Cast,
  Video,
  Loader2,
  Trash2,
} from "lucide-vue-next";

const emit = defineEmits<{
  cast: [];
  back: [];
}>();

const store = useCastStore();
const isCasting = ref(false);
const castResult = ref<string | null>(null);
const fileName = computed(() => store.selectedMedia?.split("/").pop() || "");

const recast = async () => {
  isCasting.value = true;
  castResult.value = null;

  try {
    await mediaService.castToDevice(
      store.selectedDevice!.host,
      store.selectedMedia!,
      store.castOptions!
    );
    castResult.value = "Casting to " + store.selectedDevice!.name;
    store.clearError();
  } finally {
    isCasting.value = false;
  }
};

const clearCache = async () => {
  await ClearCache();
};
</script>

<template>
  <div class="media-player h-full flex flex-col">
    <div class="flex items-center justify-between mb-4">
      <button
        @click="$emit('back')"
        class="btn-secondary flex items-center gap-2"
      >
        <ArrowLeft :size="18" />
        Back
      </button>
      <div></div>
      <div class="w-20"></div>
    </div>
    <div class="flex-1 overflow-auto space-y-6">
      <!-- Media Info -->
      <div class="flex items-center gap-4 p-4 bg-gray-700 rounded-lg">
        <div class="p-3 bg-purple-600 rounded-lg">
          <Video :size="32" />
        </div>
        <div class="flex-1 min-w-0">
          <h3 class="font-semibold text-lg truncate">{{ fileName }}</h3>
          <p class="text-sm text-gray-400 truncate">
            {{ store.selectedMedia }}
          </p>
        </div>
      </div>

      <!-- Device Info -->
      <div class="flex items-center gap-4 p-4 bg-gray-700 rounded-lg">
        <div class="p-3 bg-blue-600 rounded-lg">
          <Cast :size="32" />
        </div>
        <div class="flex-1 min-w-0">
          <h3 class="font-semibold text-lg truncate">
            {{ store.selectedDevice?.name }}
          </h3>
          <p class="text-sm text-gray-400">{{ store.selectedDevice?.type }}</p>
          <p class="text-xs text-gray-500">
            {{ store.selectedDevice?.address }}
          </p>
        </div>
      </div>
    </div>
    <!-- Recast Button -->
    <div class="flex justify-between gap-3 pt-4">
      <button @click="clearCache" class="btn-secondary flex items-center gap-2">
        <Trash2 :size="18" />
        Clear Cache
      </button>
      <div class="flex gap-3">
        <button @click="$emit('back')" class="btn-secondary">Cancel</button>
        <button
          @click="recast"
          :disabled="isCasting"
          class="btn-success flex items-center gap-2"
        >
          <Loader2 v-if="isCasting" :size="18" class="animate-spin" />
          <Cast v-else :size="18" />
          {{ isCasting ? "Casting..." : "Recast" }}
        </button>
      </div>
    </div>
  </div>
</template>
