<script setup lang="ts">
import { ref, watch } from "vue";
import { useSettingsStore } from "../stores/settings";
import { useConfirm } from "../composables/useConfirm";
import {
  Settings as SettingsIcon,
  Search,
  X,
  RotateCcw,
  Save,
  Subtitles,
  Languages,
  Brain,
  HardDrive,
  Trash2,
} from "lucide-vue-next";
import {
  GetCacheStats,
  ClearCache,
  DeleteTranscodedCache,
  DeleteAllVideoCache,
} from "../../wailsjs/go/main/App";
import { folders } from "../../wailsjs/go/models";

const settingsStore = useSettingsStore();
const showModal = defineModel<boolean>();
const { confirm } = useConfirm();

// Local copy of settings for editing
const localSettings = ref({ ...settingsStore.settings });

// Cache stats
const cacheStats = ref<folders.CacheStats | null>(null);
const loadingCache = ref(false);

// Active category for navigation
const activeCategory = ref<string>("subtitles");

const scrollToCategory = (categoryId: string) => {
  activeCategory.value = categoryId;
  const element = document.getElementById(`category-${categoryId}`);
  if (element) {
    element.scrollIntoView({ behavior: "smooth", block: "start" });
  }
};

// Watch for modal opening to create a fresh copy
watch(showModal, (isOpen) => {
  if (isOpen) {
    localSettings.value = { ...settingsStore.settings };
    loadCacheStats();
    activeCategory.value = "subtitles";
  }
});

const loadCacheStats = async () => {
  loadingCache.value = true;
  try {
    cacheStats.value = await GetCacheStats();
  } finally {
    loadingCache.value = false;
  }
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
      try {
        await ClearCache();
        await loadCacheStats();
      } catch (error) {
        console.error("Failed to delete cache:", error);
      }
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
      try {
        await DeleteTranscodedCache();
        await loadCacheStats();
      } catch (error) {
        console.error("Failed to delete transcoded cache:", error);
      }
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
      try {
        await DeleteAllVideoCache();
        await loadCacheStats();
      } catch (error) {
        console.error("Failed to delete video cache:", error);
      }
    },
  });
};

const handleReset = async () => {
  await confirm({
    title: "Reset Settings",
    message:
      "Are you sure you want to reset all settings to their default values? This action cannot be undone.",
    confirmText: "Reset",
    cancelText: "Cancel",
    variant: "danger",
    onConfirm: async () => {
      await settingsStore.resetToDefaults();
      localSettings.value = { ...settingsStore.settings };
    },
  });
};

const handleSave = async () => {
  await settingsStore.saveSettings(localSettings.value);
  showModal.value = false;
};

const handleClose = () => {
  showModal.value = false;
};

const getIconComponent = (iconName: string) => {
  switch (iconName) {
    case "Subtitles":
      return Subtitles;
    case "Languages":
      return Languages;
    case "Brain":
      return Brain;
    case "HardDrive":
      return HardDrive;
    default:
      return SettingsIcon;
  }
};
</script>

<template>
  <div v-if="showModal" class="settings-overlay" @click.self="handleClose">
    <div class="settings-modal">
      <!-- Header -->
      <div class="settings-header">
        <div class="flex items-center gap-3">
          <SettingsIcon class="w-7 h-7 text-blue-400" />
          <h2 class="settings-title">Settings</h2>
        </div>
        <button @click="handleClose" class="btn-close">
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Search Bar -->
      <div class="settings-search">
        <Search class="search-icon" />
        <input
          v-model="settingsStore.searchQuery"
          type="text"
          placeholder="Search settings..."
          class="search-input"
        />
        <button
          v-if="settingsStore.searchQuery"
          @click="settingsStore.searchQuery = ''"
          class="search-clear"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- Settings Body with Sidebar -->
      <div class="settings-body">
        <!-- Left Sidebar Navigation -->
        <div class="settings-sidebar">
          <nav class="category-nav">
            <button
              v-for="category in settingsStore.settingCategories"
              :key="category.id"
              @click="scrollToCategory(category.id)"
              :class="[
                'category-nav-item',
                { 'category-nav-item-active': activeCategory === category.id },
              ]"
            >
              <component
                :is="getIconComponent(category.icon)"
                class="w-5 h-5"
              />
              <span>{{ category.label }}</span>
            </button>
            <button
              @click="scrollToCategory('cache')"
              :class="[
                'category-nav-item',
                { 'category-nav-item-active': activeCategory === 'cache' },
              ]"
            >
              <HardDrive class="w-5 h-5" />
              <span>Cache Management</span>
            </button>
          </nav>
        </div>

        <!-- Settings Content -->
        <div class="settings-content">
          <div v-if="!settingsStore.hasSearchResults" class="settings-empty">
            <Search class="w-12 h-12 text-gray-600 mb-3" />
            <p class="text-gray-400">No settings found matching your search</p>
          </div>

          <div
            v-for="category in settingsStore.filteredCategories"
            :key="category.id"
            :id="`category-${category.id}`"
            class="settings-category"
          >
          <div class="category-header">
            <component
              :is="getIconComponent(category.icon)"
              class="w-5 h-5 text-blue-400"
            />
            <h3 class="category-title">{{ category.label }}</h3>
          </div>

          <div class="category-content">
            <div
              v-for="setting in category.settings"
              :key="setting.key"
              @click=""
              :class="`setting-item setting-item-type-${setting.type}`"
            >
              <div class="setting-info">
                <label :for="setting.key" class="setting-label">
                  {{ setting.label }}
                </label>
                <div class="setting-panel">
                  <div class="setting-control">
                    <!-- Boolean Toggle -->
                    <label
                      v-if="setting.type === 'boolean'"
                      class="toggle-switch"
                    >
                      <input
                        type="checkbox"
                        :id="setting.key"
                        v-model="localSettings[setting.key]"
                      />
                      <span class="toggle-slider"></span>
                    </label>
                    <!-- Text/Password Input -->
                    <input
                      v-else-if="
                        setting.type === 'text' || setting.type === 'password'
                      "
                      class="min-w-60"
                      :type="setting.type"
                      :id="setting.key"
                      v-model="localSettings[setting.key]"
                      :placeholder="`Enter ${setting.label.toLowerCase()}`"
                    />
                    <!-- Number Input -->
                    <div
                      v-else-if="setting.type === 'number'"
                      class="number-control"
                    >
                      <input
                        type="number"
                        :id="setting.key"
                        v-model.number="localSettings[setting.key]"
                        :min="setting.min"
                        :max="setting.max"
                        class="min-w-20"
                        :step="setting.step || 1"
                      />
                    </div>
                    <!-- Select Dropdown -->
                    <select
                      v-else-if="setting.type === 'select'"
                      :id="setting.key"
                      v-model="localSettings[setting.key]"
                      class="min-w-60"
                    >
                      <option
                        v-for="option in setting.options"
                        :key="option.value"
                        :value="option.value"
                      >
                        {{ option.label }}
                      </option>
                    </select>
                  </div>
                  <p class="setting-description">{{ setting.description }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>

          <!-- Cache Management Section -->
          <div id="category-cache" class="settings-category">
            <div class="category-header">
              <HardDrive class="w-5 h-5 text-blue-400" />
              <h3 class="category-title">Cache Management</h3>
            </div>

          <div class="category-content">
            <div class="setting-item">
              <div class="setting-info">
                <div class="setting-panel items-start!">
                  <div v-if="loadingCache" class="text-gray-400 text-sm mb-3">
                    Loading cache stats...
                  </div>
                  <div
                    v-else-if="cacheStats"
                    class="grid grid-cols-2 gap-x-6 gap-y-2 mb-3"
                  >
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
                            cacheStats.transcodedSize +
                              cacheStats.rawSegmentsSize
                          )
                        }})</span
                      >
                    </button>
                    <button
                      @click="handleDeleteAllCache"
                      class="btn-danger btn-sm w-full"
                    >
                      <Trash2 class="w-4 h-4" />
                      Delete All Cache
                      <span v-if="cacheStats" class="text-xs opacity-75 ml-auto"
                        >({{ formatBytes(cacheStats.totalSize) }})</span
                      >
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <div class="settings-footer">
        <button @click="handleReset" class="btn-secondary">
          <RotateCcw class="w-4 h-4" />
          Reset to Defaults
        </button>
        <button @click="handleSave" class="btn-primary">
          <Save class="w-4 h-4" />
          Save
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped src="./settings.css"></style>
Save" class="btn-done">
<Save class="w-4 h-4" />
Sav
