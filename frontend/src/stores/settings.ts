import { defineStore } from "pinia";
import { ref, computed } from "vue";
import {
  GetSettings,
  UpdateSettings,
  ResetSettings,
} from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";
import { settingCategories } from "../data/settingCategories";

interface SettingDefinition {
  key: keyof main.Settings;
  label: string;
  description: string;
  type: "boolean" | "text" | "password" | "number" | "select" | "textarea";
  min?: number;
  max?: number;
  step?: number;
  options?: { value: string; label: string }[];
}

export interface SettingCategory {
  id: string;
  label: string;
  icon: string;
  settings: SettingDefinition[];
}

export const useSettingsStore = defineStore("settings", () => {
  const settings = ref<main.Settings>(null!);
  const searchQuery = ref("");

  // Load settings from backend on init
  const loadSettings = async () => {
    settings.value = await GetSettings();
  };

  // Save settings to backend
  const saveSettings = async (newSettings: main.Settings) => {
    await UpdateSettings(newSettings);
    settings.value = newSettings;
  };

  // Reset to defaults
  const resetToDefaults = async () => {
    settings.value = await ResetSettings();
  };

  // Filtered settings based on search query
  const filteredCategories = computed(() => {
    if (!searchQuery.value.trim()) {
      return settingCategories;
    }

    const query = searchQuery.value.toLowerCase();
    return settingCategories
      .map((category) => ({
        ...category,
        settings: category.settings.filter(
          (setting) =>
            setting.label.toLowerCase().includes(query) ||
            setting.description.toLowerCase().includes(query) ||
            category.label.toLowerCase().includes(query)
        ),
      }))
      .filter((category) => category.settings.length > 0);
  });

  const hasSearchResults = computed(() => {
    return filteredCategories.value.length > 0;
  });

  return {
    // State
    settings,
    searchQuery,

    // Computed
    settingCategories,
    filteredCategories,
    hasSearchResults,

    // Actions
    resetToDefaults,
    loadSettings,
    saveSettings,
  };
});
