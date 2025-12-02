import { defineStore } from "pinia";
import { ref, computed } from "vue";
import {
  GetHistory,
  RemoveFromHistory,
  ClearHistory,
} from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime/runtime";

export const useHistoryStore = defineStore("history", () => {
  const items = ref<main.HistoryItem[]>([]);
  const isLoading = ref(false);
  const error = ref<string | null>(null);

  const hasHistory = computed(() => items.value.length > 0);

  // Listen for history updates from backend
  EventsOn("history:updated", (newItem: main.HistoryItem) => {
    // Remove duplicate if exists
    items.value = items.value.filter((item) => item.path !== newItem.path);
    // Add to beginning
    items.value.unshift(newItem);
  });

  const loadHistory = async () => {
    isLoading.value = true;
    error.value = null;
    try {
      items.value = await GetHistory();
    } finally {
      isLoading.value = false;
    }
  };

  const removeItem = async (path: string) => {
    await RemoveFromHistory(path);
    items.value = items.value.filter((item) => item.path !== path);
  };

  const clearAll = async () => {
    await ClearHistory();
    items.value = [];
  };

  return {
    items,
    isLoading,
    error,
    hasHistory,
    loadHistory,
    removeItem,
    clearAll,
  };
});
