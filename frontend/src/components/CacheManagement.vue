<template>
  <div class="setting-item">
    <div class="setting-panel items-start! flex">
      <div v-if="cacheStats" class="grid grid-cols-2 gap-x-6 gap-y-2 mb-3">
        <span class="text-gray-400">Total Cache:</span>
        <span class="text-white font-medium">{{
          formatBytes(cacheStats.totalSize)
        }}</span>
        <span class="text-gray-400">Transcoded:</span>
        <span class="text-white">{{
          formatBytes(cacheStats.transcodedSize)
        }}</span>
        <span class="text-gray-400">Raw:</span>
        <span class="text-white">{{
          formatBytes(cacheStats.rawSegmentsSize)
        }}</span>
        <span class="text-gray-400">Metadata:</span>
        <span class="text-white">{{
          formatBytes(cacheStats.metadataSize)
        }}</span>
      </div>
      <div class="flex flex-col gap-2">
        <button
          @click="handleDeleteTranscodedCache"
          class="btn-warning btn-sm w-full"
        >
          <Trash2 class="w-4 h-4" />
          Delete Transcoded Cache
          <span v-if="cacheStats" class="text-xs opacity-75 ml-auto"
            >({{ formatBytes(cacheStats.transcodedSize) }})</span
          >
        </button>
        <button
          @click="handleDeleteAllVideoCache"
          class="btn-warning btn-sm w-full"
        >
          <Trash2 class="w-4 h-4" />
          Delete All Video Cache
          <span v-if="cacheStats" class="text-xs opacity-75 ml-auto"
            >({{
              formatBytes(
                cacheStats.transcodedSize + cacheStats.rawSegmentsSize
              )
            }})</span
          >
        </button>
        <button @click="handleDeleteAllCache" class="btn-danger btn-sm w-full">
          <Trash2 class="w-4 h-4" />
          Delete All Cache
          <span v-if="cacheStats" class="text-xs opacity-75 ml-auto"
            >({{ formatBytes(cacheStats.totalSize) }})</span
          >
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useConfirm } from "../composables/useConfirm";
import { Trash2 } from "lucide-vue-next";
import {
  GetCacheStats,
  ClearCache,
  DeleteTranscodedCache,
  DeleteAllVideoCache,
} from "../../wailsjs/go/main/App";
import { folders } from "../../wailsjs/go/models";

const { confirm } = useConfirm();

// Cache stats
const cacheStats = ref<folders.CacheStats | null>(null);

const loadCacheStats = async () => {
  cacheStats.value = await GetCacheStats();
};

const formatBytes = (bytes: number): string => {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + " " + sizes[i];
};

const handleDeleteAllCache = async () => {
  await confirm({
    title: "Delete All Cache",
    message:
      "This will delete all cached files including metadata, video segments, and extraction data. Are you sure?",
    confirmText: "Delete All",
    cancelText: "Cancel",
    variant: "danger",
    onConfirm: async () => {
      await ClearCache();
      await loadCacheStats();
    },
  });
};

const handleDeleteTranscodedCache = async () => {
  await confirm({
    title: "Delete Transcoded Cache",
    message:
      "This will delete only transcoded video segments, keeping raw segments and metadata. Are you sure?",
    confirmText: "Delete Transcoded",
    cancelText: "Cancel",
    variant: "danger",
    onConfirm: async () => {
      await DeleteTranscodedCache();
      await loadCacheStats();
    },
  });
};

const handleDeleteAllVideoCache = async () => {
  await confirm({
    title: "Delete All Video Cache",
    message:
      "This will delete all video files (.ts segments) but keep metadata and extraction data. Are you sure?",
    confirmText: "Delete Videos",
    cancelText: "Cancel",
    variant: "danger",
    onConfirm: async () => {
      await DeleteAllVideoCache();
      await loadCacheStats();
    },
  });
};

onMounted(async () => {
  await loadCacheStats();
});
</script>
