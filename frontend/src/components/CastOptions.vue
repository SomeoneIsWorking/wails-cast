<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from "vue";
import { useCastStore } from "@/stores/cast";
import { useSettingsStore, qualityOptions } from "@/stores/settings";
import { Play } from "lucide-vue-next";
import LoadingIcon from "./LoadingIcon.vue";
import TranslationStreamModal from "./TranslationStreamModal.vue";
import { useToast } from "vue-toastification";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import { options } from "../../wailsjs/go/models";

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
const showTranslationModal = ref(false);

const handleConfirm = async () => {
  if (!trackInfo.value) return;

  const opts: options.CastOptions = {
    VideoTrack: selectedVideoTrack.value,
    AudioTrack: selectedAudioTrack.value,
    Bitrate: quality.value,
    SubtitleBurnIn: burnSubtitles.value,
    SubtitlePath:
      subtitle.value === "external"
        ? "external:" + subtitlePath.value
        : subtitle.value,
  };

  isLoading.value = true;
  try {
    await castStore.startCasting(trackInfo.value.path, opts);
    toast.success("Casting started successfully!");
  } finally {
    isLoading.value = false;
  }
};

const openTranslationModal = () => {
  showTranslationModal.value = true;
};

const hasEmbeddedSubtitles = computed(() =>
  trackInfo.value?.subtitleTracks.some((track) =>
    track.path.startsWith("embedded:")
  )
);

onMounted(() => {
  EventsOn("translation:complete", (translatedFiles: string[]) => {
    if (translatedFiles && translatedFiles.length > 0) {
      // Auto-select external subtitle with the first translated file
      subtitle.value = "external";
      subtitlePath.value = translatedFiles[0];
      toast.success(
        `Translated subtitle(s) completed!`
      );
    }
  });

  EventsOn("translation:error", (error: string) => {
    toast.error(`Translation failed: ${error}`);
  });
});

onUnmounted(() => {
  EventsOff("translation:complete");
  EventsOff("translation:error");
});
</script>

<template>
  <div class="cast-options">
    <div v-if="trackInfo">
      <div class="space-y-6 pm-2">
        <div class="flex justify-between">
          <h2 class="text-2xl font-bold text-white">Cast Options</h2>
          <button
            @click="handleConfirm"
            :disabled="isLoading"
            class="btn-primary"
          >
            <Play class="w-4 h-4" v-if="!isLoading" />
            <LoadingIcon v-else class="w-4 h-4" />
            {{ isLoading ? "Casting..." : "Start Casting" }}
          </button>
        </div>
        <!-- Video Track Selection -->
        <div>
          <h3 class="text-lg font-semibold text-white mb-2">Video Track</h3>
          <select
            v-model="selectedVideoTrack"
            class="w-full bg-gray-700 text-white rounded-md p-2"
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
            class="w-full bg-gray-700 text-white rounded-md p-2"
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
              @click="openTranslationModal"
              class="btn-success text-sm"
            >
              More Options
            </button>
          </div>

          <select
            v-model="subtitle"
            class="w-full bg-gray-700 text-white rounded-md p-2 mb-2"
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
            class="w-full bg-gray-700 text-white rounded-md p-2 mt-2"
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
            class="w-full bg-gray-700 text-white rounded-md p-2"
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
      </div>
    </div>

    <div v-else class="flex-1 flex items-center justify-center">
      <div class="py-12">
        <Play :size="64" class="text-gray-600 mx-auto mb-4" />
        <p class="text-gray-400 text-lg mb-2">No media selected</p>
        <p class="text-gray-500 text-sm mb-6">
          Select a media file to configure cast options
        </p>
      </div>
    </div>

    <TranslationStreamModal
      v-model="showTranslationModal"
    />
  </div>
</template>
