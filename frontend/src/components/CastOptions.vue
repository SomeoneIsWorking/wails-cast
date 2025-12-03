<script setup lang="ts">
import { computed, ref } from "vue";
import { useCastStore } from "@/stores/cast";
import { useSettingsStore, qualityOptions } from "@/stores/settings";
import { Play, Download, Languages } from "lucide-vue-next";
import LoadingIcon from "./LoadingIcon.vue";
import TranslationStreamModal from "./TranslationStreamModal.vue";
import {
  ExportEmbeddedSubtitles,
  TranslateExportedSubtitles,
} from "../../wailsjs/go/main/App";
import { useToast } from "vue-toastification";

const emit = defineEmits<{
  back: [];
}>();

const castStore = useCastStore();
const settingsStore = useSettingsStore();
const toast = useToast();

const trackInfo = computed(() => castStore.trackInfo);

const selectedVideoTrack = ref(0);
const selectedAudioTrack = ref(0);
const subtitlePath = ref("");
const burnSubtitles = ref(settingsStore.settings.subtitleBurnInDefault);
const quality = ref(settingsStore.settings.defaultQuality);
const subtitle = ref<string>("none");

if (trackInfo.value?.nearSubtitle) {
  subtitle.value = "external";
  subtitlePath.value = trackInfo.value.nearSubtitle;
}

const isLoading = ref(false);
const isExporting = ref(false);
const isTranslating = ref(false);
const targetLanguage = ref(settingsStore.settings.defaultTranslationLanguage);
const showTranslationModal = ref(false);

const handleConfirm = async () => {
  if (!trackInfo.value) return;

  const opts = {
    VideoTrack: selectedVideoTrack.value,
    AudioTrack: selectedAudioTrack.value,
    Bitrate: quality.value,
    MaxOutputWidth: 1920,
    Subtitle: {
      BurnIn: burnSubtitles.value,
      Path:
        subtitle.value === "external"
          ? "external:" + subtitlePath.value
          : subtitle.value,
      FontSize: settingsStore.settings.subtitleFontSize,
    },
  };

  isLoading.value = true;
  try {
    await castStore.startCasting(trackInfo.value.path, opts);
    toast.success("Casting started successfully!");
  } finally {
    isLoading.value = false;
  }
};

const handleExportSubtitles = async () => {
  if (!trackInfo.value) return;

  isExporting.value = true;
  try {
    await ExportEmbeddedSubtitles(trackInfo.value.path);
    toast.success("Subtitles exported successfully!");
  } finally {
    isExporting.value = false;
  }
};

const handleTranslateSubtitles = async () => {
  if (!trackInfo.value) return;
  if (!targetLanguage.value.trim()) {
    toast.error("Please enter a target language");
    return;
  }

  isTranslating.value = true;
  try {
    const translatedFiles = await TranslateExportedSubtitles(
      trackInfo.value.path,
      targetLanguage.value
    );
    toast.success(
      `Translated ${translatedFiles.length} subtitle(s) to ${targetLanguage.value}!`
    );
  } catch (error: any) {
    toast.error(`Translation failed: ${error.message || error}`);
  } finally {
    isTranslating.value = false;
  }
};

const openTranslationModal = () => {
  showTranslationModal.value = true;
};

const hasEmbeddedSubtitles = trackInfo.value?.subtitleTracks.some((track) =>
  track.path.startsWith("embedded:")
);
</script>

<template>
  <div class="cast-options">
    <div v-if="trackInfo">
      <div class="space-y-6 pm-2">
        <h2 class="text-2xl font-bold text-white">Cast Options</h2>

        <!-- Video Track Selection -->
        <div>
          <h3 class="text-lg font-semibold text-white mb-2">Video Track</h3>
          <select
            v-model="selectedVideoTrack"
            class="w-full bg-gray-700 text-white rounded p-2"
          >
            <option
              v-for="track in trackInfo.videoTracks"
              :key="track.index"
              :value="track.index"
            >
              Track {{ track.index }} - {{ track.codec }}
              {{ track.resolution || "" }}
            </option>
          </select>
        </div>

        <!-- Audio Track Selection -->
        <div v-if="trackInfo.audioTracks.length > 0">
          <h3 class="text-lg font-semibold text-white mb-2">Audio Track</h3>
          <select
            v-model="selectedAudioTrack"
            class="w-full bg-gray-700 text-white rounded p-2"
          >
            <option
              v-for="track in trackInfo.audioTracks"
              :key="track.index"
              :value="track.index"
            >
              Track {{ track.index }} - {{ track.language || "Unknown" }}
            </option>
          </select>
        </div>

        <!-- Subtitle Selection -->
        <div>
          <div class="flex items-center justify-between mb-2">
            <h3 class="text-lg font-semibold text-white">Subtitles</h3>
            <button
              v-if="hasEmbeddedSubtitles"
              @click="handleExportSubtitles"
              :disabled="isExporting"
              class="px-3 py-1 text-sm bg-green-600 text-white rounded hover:bg-green-700 transition disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
            >
              <Download class="w-4 h-4" />
              {{ isExporting ? "Exporting..." : "Export to WebVTT" }}
            </button>
          </div>

          <!-- Translation Section -->
          <div
            v-if="hasEmbeddedSubtitles"
            class="mb-4 p-3 bg-gray-700/50 rounded border border-gray-600"
          >
            <div class="flex items-center gap-2 mb-2">
              <Languages class="w-4 h-4 text-blue-400" />
              <h4 class="text-sm font-semibold text-white">
                Translate Exported Subtitles
              </h4>
            </div>
            <div class="flex gap-2">
              <div v-if="!isTranslating" class="flex gap-2 w-full">
                <input
                  v-model="targetLanguage"
                  type="text"
                  placeholder="Target language (e.g., Turkish)"
                  class="flex-1 bg-gray-700 text-white rounded p-2 text-sm"
                />
                <button
                  @click="handleTranslateSubtitles"
                  :disabled="!targetLanguage.trim()"
                  class="px-4 py-2 text-sm bg-blue-600 text-white rounded hover:bg-blue-700 transition disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                >
                  <Languages class="w-4 h-4" />
                  Translate
                </button>
              </div>
              <div v-else class="flex gap-2 w-full items-center">
                <div class="flex-1 bg-gray-700 text-white rounded p-2 text-sm">
                  Translating to {{ targetLanguage }}...
                </div>
                <button
                  @click="openTranslationModal"
                  class="px-4 py-2 text-sm bg-green-600 text-white rounded hover:bg-green-700 transition flex items-center gap-2"
                >
                  <LoadingIcon class="w-4 h-4" />
                  View Progress
                </button>
              </div>
            </div>
          </div>

          <select
            v-model="subtitle"
            class="w-full bg-gray-700 text-white rounded p-2 mb-2"
          >
            <option
              v-for="track in trackInfo.subtitleTracks"
              :key="track.path"
              :value="track.path"
            >
              {{ track.label }}
            </option>
          </select>

          <input
            v-if="subtitle === 'external'"
            type="text"
            v-model="subtitlePath"
            placeholder="Enter subtitle file path"
            class="w-full bg-gray-700 text-white rounded p-2 mt-2"
          />

          <div v-if="subtitle !== 'none'" class="mt-2">
            <label class="flex items-center text-white">
              <input type="checkbox" v-model="burnSubtitles" class="mr-2" />
              Burn subtitles into video
            </label>
          </div>
        </div>

        <!-- Quality Selection -->
        <div>
          <h3 class="text-lg font-semibold text-white mb-2">Quality</h3>
          <select
            v-model="quality"
            class="w-full bg-gray-700 text-white rounded p-2"
          >
            <option
              v-for="option in qualityOptions"
              :key="option.value"
              :value="option.value"
            >
              {{ option.label }}
            </option>
          </select>
        </div>

        <!-- Actions -->
        <div class="flex gap-3 justify-end pt-4 border-t border-gray-700">
          <button
            @click="$emit('back')"
            class="px-4 py-2 bg-gray-600 text-white rounded hover:bg-gray-700 transition"
          >
            Cancel
          </button>
          <button
            @click="handleConfirm"
            :disabled="isLoading"
            class="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 transition flex items-center gap-2"
          >
            <Play class="w-4 h-4" v-if="!isLoading" />
            <LoadingIcon v-else class="w-4 h-4" />
            {{ isLoading ? "Casting..." : "Start Casting" }}
          </button>
        </div>
      </div>
    </div>

    <div v-else class="flex-1 flex items-center justify-center">
      <div class="py-12">
        <Play :size="64" class="text-gray-600 mx-auto mb-4" />
        <p class="text-gray-400 text-lg mb-2">No media selected</p>
        <p class="text-gray-500 text-sm mb-6">
          Select a media file to configure cast options
        </p>
        <button @click="$emit('back')" class="btn-primary">Go to Files</button>
      </div>
    </div>

    <TranslationStreamModal
      v-model="showTranslationModal"
      :target-language="targetLanguage"
    />
  </div>
</template>
