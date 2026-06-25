<template>
  <div class="relative inline-block" ref="containerRef">
    <!-- Trigger Button -->
    <button
      @click="togglePopover"
      class="btn-icon focus:outline-none"
      title="Subtitle controls"
      @keydown.esc="showPopover = false"
    >
      <Captions :size="20" />
    </button>

    <transition
      enter-active-class="transition duration-150 ease-out"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition duration-100 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <!-- Compact single-row popover: stays short so it never clips the top of
           the window. Centered over the trigger to avoid left/right overflow. -->
      <div
        v-if="showPopover"
        class="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 p-3 bg-gray-800 border border-gray-700 rounded-xl shadow-2xl z-50 w-[440px]"
      >
        <div class="flex items-stretch gap-3">
          <!-- Font Size -->
          <div class="flex flex-col gap-1.5 flex-1 min-w-0">
            <div class="flex items-center justify-between">
              <span class="text-xs font-semibold text-gray-400 uppercase tracking-wider">Size</span>
              <span class="text-sm font-mono text-purple-400 font-bold">{{ subtitleSize }}</span>
            </div>
            <div class="flex items-center gap-2">
              <button @click="adjustSubtitleSize(-10)" class="text-gray-400 hover:text-white transition-colors" title="Decrease size">
                <Minus :size="16" />
              </button>
              <input
                type="range"
                min="10"
                max="400"
                step="10"
                :value="subtitleSize"
                @input="onSizeInput"
                class="flex-1 h-1.5 bg-gray-700 rounded-lg appearance-none cursor-pointer accent-purple-500 hover:accent-purple-400 transition-all"
              />
              <button @click="adjustSubtitleSize(10)" class="text-gray-400 hover:text-white transition-colors" title="Increase size">
                <Plus :size="16" />
              </button>
            </div>
          </div>

          <div class="w-px bg-gray-700/80"></div>

          <!-- Sync Offset (per episode) -->
          <div class="flex flex-col gap-1.5">
            <div class="flex items-center justify-between gap-4">
              <span class="text-xs font-semibold text-gray-400 uppercase tracking-wider">Sync</span>
              <span class="text-sm font-mono text-purple-400 font-bold tabular-nums">
                {{ subtitleDelay >= 0 ? "+" : "" }}{{ subtitleDelay.toFixed(1) }}s
              </span>
            </div>
            <div class="flex items-center gap-1">
              <button @click="adjustSubtitleDelay(-0.5)" class="btn-icon" title="Subtitles earlier (-0.5s)">
                <Minus :size="16" />
              </button>
              <button @click="adjustSubtitleDelay(-0.1)" class="btn-icon px-2 text-xs font-mono" title="Subtitles earlier (-0.1s)">-0.1</button>
              <button @click="resetSubtitleDelay" class="btn-icon px-2 text-xs font-mono" title="Reset offset">0</button>
              <button @click="adjustSubtitleDelay(0.1)" class="btn-icon px-2 text-xs font-mono" title="Subtitles later (+0.1s)">+0.1</button>
              <button @click="adjustSubtitleDelay(0.5)" class="btn-icon" title="Subtitles later (+0.5s)">
                <Plus :size="16" />
              </button>
            </div>
          </div>

          <div class="w-px bg-gray-700/80"></div>

          <!-- Style -->
          <div class="flex flex-col gap-1.5">
            <span class="text-xs font-semibold text-gray-400 uppercase tracking-wider">Style</span>
            <div class="flex items-center gap-2">
              <button @click="toggleBold" class="btn-icon" :class="{ 'text-purple-400 bg-purple-500/10': subtitleBold }" title="Bold">
                <Bold :size="16" />
              </button>
              <button @click="toggleItalic" class="btn-icon" :class="{ 'text-purple-400 bg-purple-500/10': subtitleItalic }" title="Italic">
                <Italic :size="16" />
              </button>
            </div>
          </div>
        </div>

        <!-- Arrow -->
        <div class="absolute top-full left-1/2 -translate-x-1/2 border-8 border-transparent border-t-gray-800"></div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from "vue";
import { Captions, Minus, Plus, Bold, Italic } from "lucide-vue-next";
import { useCastStore } from "@/stores/cast";
import { useSettingsStore } from "@/stores/settings";
import { SetSubtitleSize } from "../../wailsjs/go/main/App";

const castStore = useCastStore();
const settingsStore = useSettingsStore();
const showPopover = ref(false);
const containerRef = ref<HTMLElement | null>(null);

// Size and style are global (from saved settings); the timing offset is
// per-episode, seeded from the currently-playing media path.
const subtitleSize = ref(settingsStore.settings?.subtitleFontSize ?? 24);
const subtitleDelay = ref(
  castStore.getSubtitleDelay(castStore.playbackState.mediaPath)
);
const subtitleBold = ref(settingsStore.settings?.subtitleBold ?? false);
const subtitleItalic = ref(settingsStore.settings?.subtitleItalic ?? false);

// Re-seed the offset whenever the playing episode changes.
watch(
  () => castStore.playbackState.mediaPath,
  (path) => {
    subtitleDelay.value = castStore.getSubtitleDelay(path);
  }
);

const togglePopover = () => {
  showPopover.value = !showPopover.value;
};

const applySize = async (next: number) => {
  if (next === subtitleSize.value) return;
  subtitleSize.value = next;
  // SetSubtitleSize is the receiver's instant size message (no transcode);
  // updateLiveSubtitleSettings persists it and covers the burn-in path.
  await SetSubtitleSize(next);
  await castStore.updateLiveSubtitleSettings({ fontSize: next });
};

const adjustSubtitleSize = async (delta: number) => {
  await applySize(Math.min(400, Math.max(10, subtitleSize.value + delta)));
};

const onSizeInput = async (event: Event) => {
  await applySize(parseInt((event.target as HTMLInputElement).value, 10));
};

const adjustSubtitleDelay = async (delta: number) => {
  const next =
    Math.round(Math.min(60, Math.max(-60, subtitleDelay.value + delta)) * 10) /
    10;
  if (next === subtitleDelay.value) return;
  subtitleDelay.value = next;
  await castStore.updateLiveSubtitleSettings({ delaySeconds: next });
};

const resetSubtitleDelay = async () => {
  if (subtitleDelay.value === 0) return;
  subtitleDelay.value = 0;
  await castStore.updateLiveSubtitleSettings({ delaySeconds: 0 });
};

const toggleBold = async () => {
  subtitleBold.value = !subtitleBold.value;
  await castStore.updateLiveSubtitleSettings({ bold: subtitleBold.value });
};

const toggleItalic = async () => {
  subtitleItalic.value = !subtitleItalic.value;
  await castStore.updateLiveSubtitleSettings({ italic: subtitleItalic.value });
};

// Handle clicks outside to close popover
const handleClickOutside = (event: MouseEvent) => {
  if (
    containerRef.value &&
    !containerRef.value.contains(event.target as Node)
  ) {
    showPopover.value = false;
  }
};

onMounted(() => {
  document.addEventListener("mousedown", handleClickOutside);
});

onUnmounted(() => {
  document.removeEventListener("mousedown", handleClickOutside);
});
</script>

<style scoped>
input[type="range"]::-webkit-slider-thumb {
  appearance: none;
  width: 14px;
  height: 14px;
  background: white;
  border-radius: 50%;
  cursor: pointer;
  border: 2px solid #a855f7;
  box-shadow: 0 0 10px rgba(168, 85, 247, 0.3);
}

input[type="range"]::-moz-range-thumb {
  width: 14px;
  height: 14px;
  background: white;
  border-radius: 50%;
  cursor: pointer;
  border: 2px solid #a855f7;
}
</style>
