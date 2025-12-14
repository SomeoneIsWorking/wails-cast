<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from "vue";
import { useCastStore } from "@/stores/cast";
import { qualityOptions } from "@/stores/settings";
import { Play } from "lucide-vue-next";
import LoadingIcon from "./LoadingIcon.vue";
import TranslationStreamModal from "./TranslationStreamModal.vue";
import { useToast } from "vue-toastification";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import FileSelector from "./FileSelector.vue";

const castStore = useCastStore();
const toast = useToast();

const trackInfo = computed(() => castStore.trackInfo);

const isLoading = ref(false);
const showTranslationModal = ref(false);

const handleConfirm = async () => {
  if (!trackInfo.value) return;

  isLoading.value = true;
  try {
    await castStore.startCasting(trackInfo.value.path);
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
      if (castStore.castOptions) {
        castStore.castOptions.SubtitleType = "external";
        castStore.castOptions.SubtitlePath = translatedFiles[0];
      }
      toast.success(`Translated subtitle(s) completed!`);
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
      <div
        class="grid grid-cols-[auto_1fr] gap-3 items-center [&_label]:text-right px-5"
      >
        <label></label>
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
        <label>Video Track:</label>
        <select
          v-model="castStore.castOptions!.VideoTrack"
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
        <!-- Audio Track Selection -->
        <template v-if="trackInfo.audioTracks.length > 0">
          <label>Audio Track:</label>
          <select
            v-model="castStore.castOptions!.AudioTrack"
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
        </template>
        <!-- Subtitle Selection -->
        <label>Subtitles:</label>
        <div class="flex gap-2">
          <select
            v-model="castStore.castOptions!.SubtitleType"
            class="w-full bg-gray-700 text-white rounded-md p-2"
          >
            <option
              v-for="track in trackInfo.subtitleTracks"
              :key="track.path"
              :value="track.path"
            >
              {{ track.label }}
            </option>
          </select>
          <button
            v-if="hasEmbeddedSubtitles"
            @click="openTranslationModal"
            class="btn-success text-nowrap py-0!"
          >
            More Options
          </button>
        </div>
        <template v-if="castStore.castOptions?.SubtitleType === 'external'">
          <label></label>
          <FileSelector
            v-model="castStore.castOptions.SubtitlePath"
            placeholder="Select subtitle file"
            :accepted-extensions="['srt', 'vtt', 'ass']"
          />
        </template>
        <!-- Quality Selection -->
        <label>Quality:</label>
        <select
          v-model="castStore.castOptions!.Bitrate"
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

    <div v-else class="flex-1 flex items-center justify-center">
      <div class="py-12">
        <Play :size="64" class="text-gray-600 mx-auto mb-4" />
        <p class="text-gray-400 text-lg mb-2">No media selected</p>
        <p class="text-gray-500 text-sm mb-6">
          Select a media file to configure cast options
        </p>
      </div>
    </div>

    <TranslationStreamModal v-model="showTranslationModal" />
  </div>
</template>
