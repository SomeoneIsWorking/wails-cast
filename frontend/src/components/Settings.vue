<script setup lang="ts">
import { ref, watch, reactive } from "vue";
import { useSettingsStore } from "../stores/settings";
import { useConfirm } from "../composables/useConfirm";
import { ListModels } from "../../wailsjs/go/main/App";
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
  Smartphone,
} from "lucide-vue-next";
import CacheManagement from "./CacheManagement.vue";

const settingsStore = useSettingsStore();
const showModal = defineModel<boolean>();
const { confirm } = useConfirm();

// Textarea modal state
const textareaModal = ref<{
  show: boolean;
  key: string;
  label: string;
  value: string;
} | null>(null);

const openTextareaModal = (key: string, label: string, value: string) => {
  textareaModal.value = {
    show: true,
    key,
    label,
    value: value || "",
  };
};

const closeTextareaModal = () => {
  textareaModal.value = null;
};

const saveTextareaModal = () => {
  if (textareaModal.value) {
    (localSettings.value as any)[textareaModal.value.key] =
      textareaModal.value.value;
    closeTextareaModal();
  }
};

// Local copy of settings for editing
const localSettings = ref({ ...settingsStore.settings });

// Active category for navigation
const activeCategory = ref<string>("subtitles");

// Dynamic model options cache: keyed by provider value
const dynamicModelOptions = reactive<Record<string, string[]>>({});
const dynamicModelLoading = reactive<Record<string, boolean>>({});

/**
 * Loads model options for a given provider and caches them.
 * The cache key is the provider value so switching back doesn't re-fetch.
 */
async function loadDynamicOptions(provider: string) {
  if (dynamicModelOptions[provider] !== undefined) return; // already loaded
  dynamicModelLoading[provider] = true;
  try {
    const models = await ListModels(provider);
    dynamicModelOptions[provider] = models ?? [];
  } catch {
    dynamicModelOptions[provider] = [];
  } finally {
    dynamicModelLoading[provider] = false;
  }
}

const scrollToCategory = (categoryId: string) => {
  activeCategory.value = categoryId;
  const element = document.getElementById(`category-${categoryId}`);
  if (element) {
    element.scrollIntoView({ behavior: "smooth", block: "start" });
  }
};

// Watch for modal opening to create a fresh copy and pre-fetch models
watch(showModal, (isOpen) => {
  if (isOpen) {
    localSettings.value = { ...settingsStore.settings };
    activeCategory.value = "subtitles";
    // Pre-fetch dynamic models for current provider
    const provider = (localSettings.value as any).llmProvider ?? "opencode";
    loadDynamicOptions(provider);
  }
});

// When provider changes, load models for the new provider
watch(
  () => (localSettings.value as any).llmProvider as string,
  (newProvider) => {
    if (newProvider) {
      loadDynamicOptions(newProvider);
    }
  }
);

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
    case "Smartphone":
      return Smartphone;
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
                    <!-- Textarea Button -->
                    <button
                      v-if="setting.type === 'textarea'"
                      @click="
                        openTextareaModal(
                          setting.key,
                          setting.label,
                          localSettings[setting.key] as string
                        )
                      "
                      class="btn-secondary text-sm px-3 py-1.5"
                    >
                      View
                    </button>
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
                    <!-- Static Select Dropdown -->
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
                    <!-- Dynamic Select Dropdown (model picker) -->
                    <template v-else-if="setting.type === 'dynamic-select'">
                      <select
                        :id="setting.key"
                        v-model="localSettings[setting.key]"
                        class="min-w-60"
                        :disabled="dynamicModelLoading[(localSettings as any)[setting.dynamicOptionsProvider ?? 'llmProvider'] ?? 'opencode']"
                      >
                        <!--
                          Always include the current value so it shows even
                          if it is not in the loaded list (e.g. a custom model
                          the user typed previously). The :key ensures Vue
                          de-duplicates if it's already in the list.
                        -->
                        <option
                          v-if="localSettings[setting.key] && !(dynamicModelOptions[(localSettings as any)[setting.dynamicOptionsProvider ?? 'llmProvider'] ?? 'opencode'] ?? []).includes(localSettings[setting.key] as string)"
                          :value="localSettings[setting.key]"
                        >
                          {{ localSettings[setting.key] }}
                        </option>
                        <option
                          v-for="model in dynamicModelOptions[(localSettings as any)[setting.dynamicOptionsProvider ?? 'llmProvider'] ?? 'opencode'] ?? []"
                          :key="model"
                          :value="model"
                        >
                          {{ model }}
                        </option>
                      </select>
                      <span
                        v-if="dynamicModelLoading[(localSettings as any)[setting.dynamicOptionsProvider ?? 'llmProvider'] ?? 'opencode']"
                        class="text-xs text-gray-400 ml-2"
                      >
                        Loading…
                      </span>
                    </template>
                  </div>
                  <p class="setting-description">{{ setting.description }}</p>
                </div>
              </div>
              <CacheManagement v-if="category.id === 'cache'" />
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

    <!-- Textarea Edit Modal -->
    <div
      v-if="textareaModal?.show"
      class="settings-overlay"
      @click.self="closeTextareaModal"
    >
      <div class="settings-modal max-w-3xl">
        <div class="settings-header">
          <h2 class="settings-title">{{ textareaModal.label }}</h2>
          <button @click="closeTextareaModal" class="btn-close">
            <X class="w-5 h-5" />
          </button>
        </div>
        <div class="p-6">
          <textarea
            v-model="textareaModal.value"
            class="w-full h-96 border bg-gray-800 text-white rounded-lg p-4 font-mono text-sm resize-none focus:outline-none focus:ring-2 focus:ring-blue-500"
            :placeholder="`Enter ${textareaModal.label.toLowerCase()}`"
          />
        </div>
        <div class="settings-footer">
          <button @click="closeTextareaModal" class="btn-secondary">
            Cancel
          </button>
          <button @click="saveTextareaModal" class="btn-primary">
            <Save class="w-4 h-4" />
            Save
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped src="./settings.css"></style>
