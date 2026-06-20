<template>
  <div class="card mb-6">
    <div class="card-body">
      <!-- Header -->
      <h3 class="text-md font-bold flex gap-2 mb-2">
        <Video :size="24" class="text-purple-400 m-0.5" />
        {{ playbackState.mediaName }}
      </h3>

      <!-- Time Display -->
      <div class="flex items-center justify-between mb-2 text-sm font-mono">
        <span class="text-gray-300">{{
          formatTime(playbackState.currentTime)
        }}</span>
        <span class="text-gray-500">{{
          formatTime(playbackState.duration)
        }}</span>
      </div>

      <!-- Seek Bar -->
      <div class="relative mb-4">
        <div
          @click="onSeekBarClick"
          @mousemove="updateTooltipPosition"
          @mouseenter="showTooltip = true"
          @mouseleave="showTooltip = false"
          class="w-full h-2 bg-white/10 rounded-md cursor-pointer relative overflow-hidden"
          ref="seekBar"
        >
          <div
            class="h-full bg-linear-to-r from-purple-500 to-purple-600 rounded-md pointer-events-none transition-all duration-100"
            :style="{
              width:
                (playbackState.currentTime / playbackState.duration) * 100 +
                '%',
            }"
          ></div>
        </div>
        <div
          v-if="showTooltip"
          class="absolute bottom-full left-0 mb-2 px-2 py-1 bg-black/90 text-white text-xs rounded-md pointer-events-none whitespace-nowrap z-10"
          :style="{
            left: tooltipPosition + 'px',
            transform: 'translateX(-50%)',
          }"
        >
          {{ formatTime(seekPreviewTime) }}
        </div>
      </div>

      <!-- Controls -->
      <div class="flex items-center gap-2">
        <!-- Volume controls -->
        <VolumePopover />

        <!-- Subtitle size controls -->
        <div class="flex items-center gap-1" title="Subtitle size">
          <Captions :size="18" class="text-gray-400" />
          <button
            @click="adjustSubtitleSize(-2)"
            class="btn-icon"
            title="Decrease subtitle size"
          >
            <Minus :size="16" />
          </button>
          <span class="text-xs font-mono text-gray-300 w-6 text-center">{{
            subtitleSize
          }}</span>
          <button
            @click="adjustSubtitleSize(2)"
            class="btn-icon"
            title="Increase subtitle size"
          >
            <Plus :size="16" />
          </button>
        </div>

        <div class="flex-1"></div>
        <button @click="seekRelative(-30)" class="btn-icon" title="Rewind 30s">
          <Rewind :size="18" />
        </button>
        <button @click="seekRelative(-10)" class="btn-icon" title="Rewind 10s">
          <SkipBack :size="18" />
        </button>
        <button @click="togglePause" class="btn-success">
          <Play v-if="playbackState.status === 'PAUSED'" :size="18" />
          <Pause v-else :size="18" />
          {{ playbackState.status === "PAUSED" ? "Play" : "Pause" }}
        </button>
        <button @click="seekRelative(10)" class="btn-icon" title="Forward 10s">
          <SkipForward :size="18" />
        </button>
        <button @click="seekRelative(30)" class="btn-icon" title="Forward 30s">
          <FastForward :size="18" />
        </button>
        <div class="flex-1 flex justify-end">
          <button @click="stopPlayback" class="btn-danger">
            <Square :size="18" />
            Stop
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from "vue";
import { mediaService } from "../services/media";
import VolumePopover from "./VolumePopover.vue";
import {
  Video,
  Pause,
  Play,
  Square,
  Rewind,
  FastForward,
  SkipBack,
  SkipForward,
  Captions,
  Minus,
  Plus,
} from "lucide-vue-next";
import { useCastStore } from "@/stores/cast";
import { useSettingsStore } from "@/stores/settings";
import { SetSubtitleSize } from "../../wailsjs/go/main/App";
import { watch } from "vue";

const seekPosition = ref(0);
const showTooltip = ref(false);
const tooltipPosition = ref(0);
const seekPreviewTime = ref(0);
const seekBar = ref<HTMLInputElement | null>(null);
let updateInterval: ReturnType<typeof setInterval> | null = null;
let localTimeIncrement: ReturnType<typeof setInterval> | null = null;
const castStore = useCastStore();
const settingsStore = useSettingsStore();

const playbackState = computed(() => castStore.playbackState);

// Live subtitle size, seeded from the configured default font size.
const subtitleSize = ref(settingsStore.settings?.subtitleFontSize ?? 24);

const adjustSubtitleSize = async (delta: number) => {
  const next = Math.min(80, Math.max(8, subtitleSize.value + delta));
  if (next === subtitleSize.value) return;
  subtitleSize.value = next;
  await SetSubtitleSize(next);
};

watch(
  () => playbackState.value.status,
  (status) => {
    if (status === "PLAYING") {
      startLocalTimeIncrement();
    } else {
      if (localTimeIncrement) {
        clearInterval(localTimeIncrement);
      }
    }
  }
);

// Update seek preview when hovering
const updateTooltipPosition = (event: MouseEvent) => {
  if (!seekBar.value || !playbackState.value.duration) return;

  const rect = seekBar.value.getBoundingClientRect();
  const percent = (event.clientX - rect.left) / rect.width;
  const time = Math.max(
    0,
    Math.min(
      playbackState.value.duration,
      percent * playbackState.value.duration
    )
  );

  seekPreviewTime.value = Math.floor(time);
  tooltipPosition.value = event.clientX - rect.left;
};

// Handle click on seek bar
const onSeekBarClick = async (event: MouseEvent) => {
  if (!seekBar.value || !playbackState.value.duration) return;

  const rect = seekBar.value.getBoundingClientRect();
  const percent = (event.clientX - rect.left) / rect.width;
  const time = Math.max(
    0,
    Math.min(
      playbackState.value.duration,
      percent * playbackState.value.duration
    )
  );

  seekPosition.value = Math.floor(time);
  await onSeek();
};

// Increment local time every second when playing
const startLocalTimeIncrement = () => {
  if (localTimeIncrement) {
    clearInterval(localTimeIncrement);
  }

  localTimeIncrement = setInterval(() => {
    if (playbackState.value.status === "PLAYING") {
      playbackState.value.currentTime += 1;
      // Increment local position
      seekPosition.value = playbackState.value.currentTime;
    }
  }, 1000);
};

// Toggle pause/play
const togglePause = async () => {
  if (playbackState.value.status === "PAUSED") {
    await mediaService.unpause();
  } else {
    await mediaService.pause();
  }
};

// Seek to position
const onSeek = async () => {
  await mediaService.seekTo(seekPosition.value);
};

// Seek relative
const seekRelative = async (seconds: number) => {
  const newTime = Math.max(
    0,
    Math.min(
      playbackState.value.duration,
      playbackState.value.currentTime + seconds
    )
  );
  seekPosition.value = newTime;
  await onSeek();
};

// Stop playback
const stopPlayback = async () => {
  await mediaService.stopPlayback();
  if (updateInterval) clearInterval(updateInterval);
};

// Format time in MM:SS or HH:MM:SS
const formatTime = (seconds: number) => {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);

  if (h > 0) {
    return `${h}:${m.toString().padStart(2, "0")}:${s
      .toString()
      .padStart(2, "0")}`;
  }
  return `${m}:${s.toString().padStart(2, "0")}`;
};

onMounted(() => {
  if (playbackState.value.status === "PLAYING") {
    startLocalTimeIncrement();
  }
});
onUnmounted(() => {
  if (localTimeIncrement) {
    clearInterval(localTimeIncrement);
  }
});
</script>
