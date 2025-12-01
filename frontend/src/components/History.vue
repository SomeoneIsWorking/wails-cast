<script setup lang="ts">
import { onMounted } from "vue";
import { useHistoryStore } from "../stores/history";
import { Play, X } from "lucide-vue-next";

const emit = defineEmits<{
  select: [path: string];
}>();

const historyStore = useHistoryStore();

onMounted(() => {
  historyStore.loadHistory();
});

const formatDate = (timestamp: string) => {
  const date = new Date(timestamp);
  const now = new Date();
  const diff = now.getTime() - date.getTime();

  // Less than 1 minute
  if (diff < 60000) return "Just now";

  // Less than 1 hour
  if (diff < 3600000) {
    const mins = Math.floor(diff / 60000);
    return `${mins} minute${mins > 1 ? "s" : ""} ago`;
  }

  // Less than 24 hours
  if (diff < 86400000) {
    const hours = Math.floor(diff / 3600000);
    return `${hours} hour${hours > 1 ? "s" : ""} ago`;
  }

  // Same year
  if (date.getFullYear() === now.getFullYear()) {
    return date.toLocaleDateString("en-US", { month: "short", day: "numeric" });
  }

  return date.toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
};

const handleSelect = (path: string) => {
  emit("select", path);
};

const handleRemove = async (e: Event, path: string) => {
  e.stopPropagation();
  await historyStore.removeItem(path);
};

const handleClearAll = async () => {
  await historyStore.clearAll();
};
</script>

<template>
  <div class="cast-history h-full flex flex-col">
    <div
      class="flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700"
    >
      <h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
        Recent Casts
      </h2>
      <button
        v-if="historyStore.hasHistory"
        @click="handleClearAll"
        class="text-sm text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 transition-colors"
      >
        Clear All
      </button>
    </div>

    <div
      v-if="historyStore.isLoading"
      class="flex-1 flex items-center justify-center p-8"
    >
      <p class="text-gray-500 dark:text-gray-400">Loading history...</p>
    </div>

    <div
      v-else-if="!historyStore.hasHistory"
      class="flex-1 flex items-center justify-center p-8"
    >
      <p class="text-gray-500 dark:text-gray-400 text-center">
        No recent casts. Start casting to see your history here.
      </p>
    </div>

    <div v-else class="flex-1 overflow-y-auto">
      <div
        v-for="item in historyStore.items"
        :key="item.path"
        class="group border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors cursor-pointer"
      >
        <div class="p-4 flex items-center gap-3">
          <button
            @click="handleSelect(item.path)"
            class="flex-1 min-w-0 text-left"
          >
            <div class="flex items-center gap-2 mb-1">
              <Play class="h-4 w-4 text-blue-600 dark:text-blue-400 shrink-0" />
              <h3 class="font-medium text-gray-900 dark:text-gray-100 truncate">
                {{ item.name }}
              </h3>
            </div>
            <div
              class="flex items-center gap-3 text-sm text-gray-500 dark:text-gray-400"
            >
              <span>{{ item.deviceName }}</span>
              <span>•</span>
              <span>{{ formatDate(item.timestamp) }}</span>
            </div>
            <p class="text-xs text-gray-400 dark:text-gray-500 truncate mt-1">
              {{ item.path }}
            </p>
          </button>

          <button
            @click="(e) => handleRemove(e, item.path)"
            class="p-2 text-gray-400 hover:text-red-600 dark:hover:text-red-400 opacity-0 group-hover:opacity-100 transition-all shrink-0"
            title="Remove from history"
          >
            <X class="h-4 w-4" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
