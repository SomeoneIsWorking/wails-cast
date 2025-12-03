<script setup lang="ts">
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
} from "lucide-vue-next";

const settingsStore = useSettingsStore();
const showModal = defineModel<boolean>();
const { confirm } = useConfirm();

const handleReset = async () => {
  await confirm({
    title: "Reset Settings",
    message: "Are you sure you want to reset all settings to their default values? This action cannot be undone.",
    confirmText: "Reset",
    cancelText: "Cancel",
    variant: "danger",
    onConfirm: async () => {
      await settingsStore.resetToDefaults();
    },
  });
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

      <!-- Settings Content -->
      <div class="settings-content">
        <div v-if="!settingsStore.hasSearchResults" class="settings-empty">
          <Search class="w-12 h-12 text-gray-600 mb-3" />
          <p class="text-gray-400">No settings found matching your search</p>
        </div>

        <div
          v-for="category in settingsStore.filteredCategories"
          :key="category.id"
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
                        :checked="(settingsStore.settings[setting.key] as boolean)"
                        @change="
                          settingsStore.updateSetting(
                            setting.key,
                            ($event.target as HTMLInputElement).checked
                          )
                        "
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
                      :value="settingsStore.settings[setting.key]"
                      @input="
                        settingsStore.updateSetting(
                          setting.key,
                          ($event.target as HTMLInputElement).value
                        )
                      "
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
                        :value="settingsStore.settings[setting.key]"
                        :min="setting.min"
                        :max="setting.max"
                        class="min-w-20"
                        :step="setting.step || 1"
                        @input="
                          settingsStore.updateSetting(
                            setting.key,
                            Number(($event.target as HTMLInputElement).value)
                          )
                        "
                      />
                    </div>
                    <!-- Select Dropdown -->
                    <select
                      v-else-if="setting.type === 'select'"
                      :id="setting.key"
                      :value="settingsStore.settings[setting.key]"
                      class="min-w-60"
                      @change="
                        settingsStore.updateSetting(
                          setting.key,
                          ($event.target as HTMLSelectElement).value
                        )
                      "
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
      </div>

      <!-- Footer -->
      <div class="settings-footer">
        <button @click="handleReset" class="btn-reset">
          <RotateCcw class="w-4 h-4" />
          Reset to Defaults
        </button>
        <button @click="handleClose" class="btn-done">
          <Save class="w-4 h-4" />
          Done
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped src="./settings.css"></style>
