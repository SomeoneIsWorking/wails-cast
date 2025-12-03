import { defineStore } from "pinia";
import { ref, computed } from "vue";
import {
  GetSettings,
  UpdateSettings,
  ResetSettings,
} from "../../wailsjs/go/main/App";

export interface Settings {
  subtitleBurnInDefault: boolean;
  defaultTranslationLanguage: string;
  geminiApiKey: string;
  geminiModel: string;
  defaultQuality: string;
  subtitleFontSize: number;
}

interface SettingDefinition {
  key: keyof Settings;
  label: string;
  description: string;
  type: "boolean" | "text" | "password" | "number" | "select";
  min?: number;
  max?: number;
  step?: number;
  options?: { value: string; label: string }[];
}

interface SettingCategory {
  id: string;
  label: string;
  icon: string;
  settings: SettingDefinition[];
}

export const qualityOptions = [
  { value: "", label: "Original (Best Quality)" },
  { value: "8M", label: "Very High (Bitrate: 8M)" },
  { value: "5M", label: "High (Bitrate: 5M)" },
  { value: "3M", label: "Medium (Bitrate: 3M)" },
  { value: "2M", label: "Low (Bitrate: 2M)" },
];

export const useSettingsStore = defineStore("settings", () => {
  const settings = ref<Settings>(null!);
  const searchQuery = ref("");

  // Load settings from backend on init
  const loadSettings = async () => {
    settings.value = await GetSettings();
  };

  // Save settings to backend
  const saveSettings = async (newSettings: Settings) => {
    await UpdateSettings(newSettings);
    settings.value = newSettings;
  };

  // Reset to defaults
  const resetToDefaults = async () => {
    settings.value = await ResetSettings();
  };

  // Setting categories for organization
  const settingCategories = computed<SettingCategory[]>(() => [
    {
      id: "subtitles",
      label: "Subtitles",
      icon: "Subtitles",
      settings: [
        {
          key: "subtitleBurnInDefault",
          label: "Burn-in Subtitles by Default",
          description: "Automatically burn subtitles into video stream",
          type: "boolean",
        },
        {
          key: "subtitleFontSize",
          label: "Font Size",
          description: "Default font size for burned-in subtitles",
          type: "number",
          min: 12,
          max: 72,
          step: 2,
        },
      ],
    },
    {
      id: "translation",
      label: "Translation",
      icon: "Languages",
      settings: [
        {
          key: "defaultTranslationLanguage",
          label: "Default Target Language",
          description: "Default language for subtitle translation",
          type: "text",
        },
      ],
    },
    {
      id: "ai",
      label: "AI Configuration",
      icon: "Brain",
      settings: [
        {
          key: "geminiApiKey",
          label: "Gemini API Key",
          description: "Your Google Gemini API key for AI features",
          type: "password",
        },
        {
          key: "geminiModel",
          label: "Gemini Model",
          description: "Which Gemini model to use",
          type: "text",
        },
      ],
    },
    {
      id: "quality",
      label: "Quality",
      icon: "Settings",
      settings: [
        {
          key: "defaultQuality",
          label: "Default Quality",
          description: "Default quality preset for video encoding",
          type: "select",
          options: qualityOptions,
        },
      ],
    },
  ]);

  // Filtered settings based on search query
  const filteredCategories = computed(() => {
    if (!searchQuery.value.trim()) {
      return settingCategories.value;
    }

    const query = searchQuery.value.toLowerCase();
    return settingCategories.value
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
