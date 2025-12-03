<script setup lang="ts">
import { ref } from "vue";
import { options, type main } from "../../wailsjs/go/models";
import { CastOptions, mediaService } from "@/services/media";
import { useCastStore } from "@/stores/cast";
import { useSettingsStore } from "@/stores/settings";
import { Play, Download, Languages } from "lucide-vue-next";
import LoadingIcon from "./LoadingIcon.vue";
import TranslationStreamModal from "./TranslationStreamModal.vue";
import { ExportEmbeddedSubtitles, TranslateExportedSubtitles } from "../../wailsjs/go/main/App";
import { useToast } from "vue-toastification";

const props = defineProps<{
  trackInfo: main.TrackDisplayInfo;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: boolean];
  confirm: [options: options.CastOptions];
}>();

const castStore = useCastStore();
const settingsStore = useSettingsStore();
const toast = useToast();

const selectedVideoTrack = ref(0);
const selectedAudioTrack = ref(0);
const subtitlePath = ref("");
const burnSubtitles = ref(settingsStore.settings.subtitleBurnInDefault);
const qualityOptions = await mediaService.getQualityOptions();
const quality = ref(
  settingsStore.settings.defaultQuality || 
  (qualityOptions.find((x) => x.Default) || qualityOptions[0]).Key
);
const subtitle = ref<string>("none");

if (props.trackInfo.nearSubtitle) {
  subtitle.value = "external";
  subtitlePath.value = props.trackInfo.nearSubtitle;
}

const showDialog = defineModel<boolean>();
const isLoading = ref(false);
const isExporting = ref(false);
const isTranslating = ref(false);
const targetLanguage = ref(settingsStore.settings.defaultTranslationLanguage);
const showTranslationModal = ref(false);

const handleConfirm = async () => {
  const opts = {
    VideoTrack: selectedVideoTrack.value,
    AudioTrack: selectedAudioTrack.value,
    Bitrate: quality.value,
    Subtitle: {
      BurnIn: burnSubtitles.value,
      Path:
        subtitle.value === "external"
          ? "external:" + subtitlePath.value
          : subtitle.value,
    },
  } as CastOptions;

  isLoading.value = true;
  try {
    await castStore.startCasting(props.trackInfo.path, opts);
    showDialog.value = false;
    toast.success("Casting started successfully!");
  } finally {
    isLoading.value = false;
  }
};

const handleExportSubtitles = async () => {
  isExporting.value = true;
  try {
    await ExportEmbeddedSubtitles(props.trackInfo.path);
    toast.success("Subtitles exported successfully!");
  } finally {
    isExporting.value = false;
  }
};

const handleTranslateSubtitles = async () => {
  if (!targetLanguage.value.trim()) {
    toast.error("Please enter a target language");
    return;
  }
  
  isTranslating.value = true;
  try {
    const translatedFiles = await TranslateExportedSubtitles(
      props.trackInfo.path,
      targetLanguage.value,
    );
    toast.success(`Translated ${translatedFiles.length} subtitle(s) to ${targetLanguage.value}!`);
  } catch (error: any) {
    toast.error(`Translation failed: ${error.message || error}`);
  } finally {
    isTranslating.value = false;
  }
};

const openTranslationModal = () => {
  showTranslationModal.value = true;
};

const handleCancel = () => {
  showDialog.value = false;
};

const hasEmbeddedSubtitles = props.trackInfo.subtitleTracks.some((track) =>
  track.path.startsWith("embedded:")
);
</script>

<template>
  <div
    v-if="showDialog"
    class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
    @click.self="handleCancel"
  >
    <div
      class="bg-gray-800 rounded-lg p-6 max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto"
    >
      <h2 class="text-2xl font-bold text-white mb-4">Select Tracks</h2>

      <!-- Video Track Selection -->
      <div class="mb-6">
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
      <div v-if="trackInfo.audioTracks.length > 0" class="mb-6">
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
      <div class="mb-6">
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
        <div v-if="hasEmbeddedSubtitles" class="mb-4 p-3 bg-gray-700/50 rounded border border-gray-600">
          <div class="flex items-center gap-2 mb-2">
            <Languages class="w-4 h-4 text-blue-400" />
            <h4 class="text-sm font-semibold text-white">Translate Exported Subtitles</h4>
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
      <div class="mb-6">
        <h3 class="text-lg font-semibold text-white mb-2">Quality</h3>
        <select
          v-model="quality"
          class="w-full bg-gray-700 text-white rounded p-2"
        >
          <option
            v-for="option in qualityOptions"
            :key="option.Key"
            :value="option.Key"
          >
            {{ option.Label }}
          </option>
        </select>
      </div>

      <!-- Actions -->
      <div class="flex gap-3 justify-end">
        <button
          @click="handleCancel"
          class="px-4 py-2 bg-gray-600 text-white rounded hover:bg-gray-700 transition"
        >
          Cancel
        </button>
        <button
          @click="handleConfirm"
          :disabled="isLoading"
          class="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 transition"
        >
          <Play class="inline-block w-4 h-4 mr-2" v-if="!isLoading" />
          <LoadingIcon v-else class="inline-block w-4 h-4 mr-2" />
          {{ isLoading ? "Casting..." : "Cast" }}
        </button>
      </div>
    </div>
  </div>

  <TranslationStreamModal
    v-model="showTranslationModal"
    :target-language="targetLanguage"
  />
</template>
