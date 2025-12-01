<script setup lang="ts">
import { ref } from "vue";
import { options, type main } from "../../wailsjs/go/models";
import { CastOptions, mediaService } from "@/services/media";
import { useCastStore } from "@/stores/cast";

const props = defineProps<{
  trackInfo: main.TrackDisplayInfo;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: boolean];
  confirm: [options: options.CastOptions];
}>();

const selectedVideoTrack = ref(0);
const selectedAudioTrack = ref(0);
const subtitleSource = ref<"none" | "embedded" | "external">("none");
const externalSubtitlePath = ref("");
const burnSubtitles = ref(false);
const qualityOptions = await mediaService.getQualityOptions();
const quality = ref(qualityOptions[0].CRF);
const subtitle = ref<string>("none");
const showDialog = defineModel<boolean>();
const castStore = useCastStore();
const isLoading = ref(false);

const handleConfirm = async () => {
  const opts = {
    Stream: {
      VideoTrack: selectedVideoTrack.value,
      AudioTrack: selectedAudioTrack.value,
      CRF: quality.value,
      Subtitle: {
        BurnIn: burnSubtitles.value,
        Path:
          subtitle.value === "external"
            ? "external:" + externalSubtitlePath.value
            : subtitle.value,
      },
    },
    Debug: true,
    NoCastJustHost: false,
  } as CastOptions;

  isLoading.value = true;
  try {
    await castStore.startCasting(props.trackInfo.path, opts);
    showDialog.value = false;
  } finally {
    isLoading.value = false;
  }
};

const handleCancel = () => {
  showDialog.value = false;
};
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
        <h3 class="text-lg font-semibold text-white mb-2">Subtitles</h3>
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

        <div v-if="subtitleSource !== 'none'" class="mt-2">
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
            :value="option.CRF"
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
          v-if="!isLoading"
          class="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 transition"
        >
          Start Casting
        </button>
        <div
          v-else
          class="px-4 py-2 bg-blue-600 text-white rounded flex items-center justify-center"
        >
          <svg
            class="animate-spin h-5 w-5 mr-2 text-white"
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            ></circle>
          </svg>
          Casting...
        </div>
      </div>
    </div>
  </div>
</template>
