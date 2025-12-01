import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { GetHistory, RemoveFromHistory, ClearHistory } from "../../wailsjs/go/main/App";
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
    } catch (err) {
      error.value = "Failed to load history";
      console.error("Failed to load history:", err);
    } finally {
      isLoading.value = false;
    }
  };

  const removeItem = async (path: string) => {
    try {
      await RemoveFromHistory(path);
      items.value = items.value.filter((item) => item.path !== path);
    } catch (err) {
      error.value = "Failed to remove item";
      console.error("Failed to remove from history:", err);
      throw err;
    }
  };

  const clearAll = async () => {
    try {
      await ClearHistory();
      items.value = [];
    } catch (err) {
      error.value = "Failed to clear history";
      console.error("Failed to clear history:", err);
      throw err;
    }
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
