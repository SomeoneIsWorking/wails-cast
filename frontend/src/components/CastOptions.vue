<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from "vue";
import { useCastStore } from "@/stores/cast";
import { Cloud, Play } from "lucide-vue-next";
import LoadingIcon from "./LoadingIcon.vue";
import TranslationStreamModal from "./TranslationStreamModal.vue";
import { useToast } from "vue-toastification";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import FileSelector from "./FileSelector.vue";
import { qualityOptions } from "@/data/qualityOptions";
import { OpenMediaFolder } from "../../wailsjs/go/main/App";
import TrackDownloader from "./TrackDownloader.vue";

const castStore = useCastStore();
const toast = useToast();

const trackInfo = computed(() => castStore.trackInfo);

const isLoading = ref(false);
const showTranslationModal = ref(false);

const handleConfirm = async () => {
  if (!trackInfo.value) return;

  isLoading.value = true;
  try {
    await castStore.startCasting(trackInfo.value.Path);
    toast.success("Casting started successfully!");
  } finally {
    isLoading.value = false;
  }
};

const openTranslationModal = () => {
  showTranslationModal.value = true;
};

const openCacheFolder = async () => {
  if (!trackInfo.value) return;
  await OpenMediaFolder(trackInfo.value.Path);
};

const hasEmbeddedSubtitles = computed(() =>
  trackInfo.value?.SubtitleTracks.some((track) =>
    track.Path.startsWith("embedded:")
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
        class="grid grid-cols-[auto_1fr] gap-3 items-start [&>label]:text-right [&>label]:py-1 px-5"
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
        <div>
          <TrackDownloader
            :path="trackInfo.Path"
            type="video"
            :track="castStore.castOptions!.VideoTrack"
          >
            <select
              v-model="castStore.castOptions!.VideoTrack"
              class="flex-1 bg-gray-700 text-white rounded-md p-2"
            >
              <option
                v-for="track in trackInfo.VideoTracks"
                :key="track.Index"
                :value="track.Index"
              >
                Track
                {{ track.Index }} - {{ track.Codecs || "Unknown" }} -
                {{ track.Resolution || "" }}
              </option>
            </select>
          </TrackDownloader>
        </div>

        <!-- Audio Track Selection -->
        <template v-if="trackInfo.AudioTracks.length > 0">
          <label>Audio Track:</label>
          <TrackDownloader
            :path="trackInfo.Path"
            type="audio"
            :track="castStore.castOptions!.AudioTrack"
          >
            <select
              v-model="castStore.castOptions!.AudioTrack"
              class="flex-1 bg-gray-700 text-white rounded-md p-2"
            >
              <option
                v-for="track in trackInfo.AudioTracks"
                :key="track.Index"
                :value="track.Index"
              >
                Track {{ track.Index }} - {{ track.Language || "Unknown" }}
              </option>
            </select>
          </TrackDownloader>
        </template>
        <!-- Subtitle Selection -->
        <label>Subtitles:</label>
        <div class="flex gap-2">
          <select
            v-model="castStore.castOptions!.SubtitleType"
            class="w-full bg-gray-700 text-white rounded-md p-2"
          >
            <option
              v-for="track in trackInfo.SubtitleTracks"
              :key="track.Path"
              :value="track.Path"
            >
              {{ track.Label }}
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
        <label></label>
        <div class="flex justify-end gap-2">
          <button @click="openCacheFolder" class="btn-secondary">
            <Cloud></Cloud>
            Open Cache Folder
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
      </div>
    </div>

    <TranslationStreamModal v-model="showTranslationModal" />
  </div>
</template>
