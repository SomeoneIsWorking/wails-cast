<template>
  <div class="relative inline-block" ref="containerRef">
    <!-- Trigger Button -->
    <button 
      @click="togglePopover" 
      class="btn-icon focus:outline-none" 
      title="Volume controls"
      @keydown.esc="showPopover = false"
    >
      <VolumeX v-if="playbackState.muted || playbackState.volume === 0" :size="20" class="text-red-400" />
      <Volume2 v-else :size="20" />
    </button>

    <transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="translate-y-1 opacity-0"
      enter-to-class="translate-y-0 opacity-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="translate-y-0 opacity-100"
      leave-to-class="translate-y-1 opacity-0"
    >
      <div 
        v-if="showPopover" 
        class="absolute bottom-full left-0 mb-2 p-4 bg-gray-800 border border-gray-700 rounded-xl shadow-2xl z-50 min-w-[240px]"
      >
        <div class="flex flex-col gap-4">
          <!-- Volume Info -->
          <div class="flex items-center justify-between">
            <span class="text-xs font-semibold text-gray-400 uppercase tracking-wider">Volume</span>
            <span class="text-sm font-mono text-blue-400 font-bold">
              {{ Math.round(playbackState.volume * 100) }}%
            </span>
          </div>

          <!-- Slider -->
          <div class="flex items-center gap-3">
            <button @click="volumeDown" class="text-gray-400 hover:text-white transition-colors">
              <Volume1 :size="16" />
            </button>
            
            <div class="flex-1 h-6 flex items-center">
              <input 
                type="range" 
                min="0" 
                max="1" 
                step="0.01" 
                :value="playbackState.volume" 
                @input="updateVolume"
                class="w-full h-1.5 bg-gray-700 rounded-lg appearance-none cursor-pointer accent-blue-500 hover:accent-blue-400 transition-all"
              />
            </div>

            <button @click="volumeUp" class="text-gray-400 hover:text-white transition-colors">
              <Volume2 :size="16" />
            </button>
          </div>
        </div>
        
        <!-- Arrow -->
        <div class="absolute top-full left-4 border-8 border-transparent border-t-gray-800"></div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";
import { Volume1, Volume2, VolumeX } from "lucide-vue-next";
import { useCastStore } from "@/stores/cast";
import { mediaService } from "@/services/media";

const castStore = useCastStore();
const playbackState = computed(() => castStore.playbackState);
const showPopover = ref(false);
const containerRef = ref<HTMLElement | null>(null);

const togglePopover = () => {
  showPopover.value = !showPopover.value;
};

const updateVolume = async (event: Event) => {
  const value = parseFloat((event.target as HTMLInputElement).value);
  await mediaService.setVolume(value);
};

const setVolume = async (value: number) => {
  await mediaService.setVolume(value);
};

const toggleMute = async () => {
  await mediaService.setMuted(!playbackState.value.muted);
};

const volumeUp = async () => {
  const newVolume = Math.min(1, playbackState.value.volume + 0.01);
  await mediaService.setVolume(newVolume);
};

const volumeDown = async () => {
  const newVolume = Math.max(0, playbackState.value.volume - 0.01);
  await mediaService.setVolume(newVolume);
};

// Handle clicks outside to close popover
const handleClickOutside = (event: MouseEvent) => {
  if (containerRef.value && !containerRef.value.contains(event.target as Node)) {
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
  border: 2px solid #3b82f6;
  box-shadow: 0 0 10px rgba(59, 130, 246, 0.3);
}

input[type="range"]::-moz-range-thumb {
  width: 14px;
  height: 14px;
  background: white;
  border-radius: 50%;
  cursor: pointer;
  border: 2px solid #3b82f6;
}
</style>
