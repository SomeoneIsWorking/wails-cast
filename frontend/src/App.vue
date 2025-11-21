<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useCastStore } from "./stores/cast";
import DeviceDiscovery from "./components/DeviceDiscovery.vue";
import MediaPlayer from "./components/MediaPlayer.vue";
import FileExplorer from "./components/FileExplorer.vue";
import PlaybackControl from "./components/PlaybackControl.vue";
import { Tv, Video, Play, AlertCircle, X } from "lucide-vue-next";

const store = useCastStore();
const activeTab = ref<"devices" | "files" | "player">("devices");

onMounted(() => {
  store.discoverDevices();
});

const selectDevice = (device: any) => {
  store.selectDevice(device);
  activeTab.value = "files";
};

const selectMedia = () => {
  activeTab.value = "player";
};

const handleCast = () => {
  activeTab.value = "player";
};
</script>

<template>
  <div
    class="min-h-screen bg-linear-to-br from-gray-900 via-gray-800 to-gray-900 text-white"
  >
    <div class="container mx-auto px-4 py-6 max-w-6xl">
      <!-- Header -->
      <header class="mb-6">
        <h1
          class="text-4xl font-bold bg-linear-to-r from-blue-400 to-purple-500 bg-clip-text text-transparent"
        >
          Wails Cast
        </h1>
        <p class="text-gray-400 mt-1">Cast your local videos to any device</p>
      </header>

      <!-- Playback Control (shown when something is playing) -->
      <PlaybackControl />

      <main class="mt-6">
        <!-- Tab Navigation -->
        <div class="flex gap-2 mb-6 border-b border-gray-700">
          <button
            :class="[
              'px-4 py-2 font-medium transition-all duration-200 border-b-2',
              activeTab === 'devices'
                ? 'border-blue-500 text-blue-400'
                : 'border-transparent text-gray-400 hover:text-gray-200',
            ]"
            @click="activeTab = 'devices'"
          >
            <span class="flex items-center gap-2">
              <Tv :size="18" /> Devices
            </span>
          </button>
          <button
            v-if="store.hasSelectedDevice"
            :class="[
              'px-4 py-2 font-medium transition-all duration-200 border-b-2',
              activeTab === 'files'
                ? 'border-blue-500 text-blue-400'
                : 'border-transparent text-gray-400 hover:text-gray-200',
            ]"
            @click="activeTab = 'files'"
          >
            <span class="flex items-center gap-2">
              <Video :size="18" /> Media Files
            </span>
          </button>
          <button
            v-if="store.hasSelectedMedia"
            :class="[
              'px-4 py-2 font-medium transition-all duration-200 border-b-2',
              activeTab === 'player'
                ? 'border-blue-500 text-blue-400'
                : 'border-transparent text-gray-400 hover:text-gray-200',
            ]"
            @click="activeTab = 'player'"
          >
            <span class="flex items-center gap-2">
              <Play :size="18" /> Cast
            </span>
          </button>
        </div>

        <!-- Error Message -->
        <div
          v-if="store.error"
          class="mb-6 bg-red-900/50 border border-red-700 rounded-lg p-4 flex items-center justify-between"
        >
          <span class="flex items-center gap-2">
            <AlertCircle :size="20" /> {{ store.error }}
          </span>
          <button
            @click="store.clearError"
            class="text-red-400 hover:text-red-300"
          >
            <X :size="20" />
          </button>
        </div>

        <!-- Device Discovery Tab -->
        <section class="h-full" v-show="activeTab === 'devices'">
          <DeviceDiscovery @select="selectDevice" />
        </section>

        <!-- Media Files Tab -->
        <section class="h-full" v-show="activeTab === 'files'">
          <div
            v-if="store.hasSelectedDevice"
            class="mb-4 p-3 bg-gray-800 rounded-lg border border-gray-700"
          >
            <strong class="text-gray-300">Selected Device:</strong>
            <span class="text-blue-400">{{ store.selectedDevice?.name }}</span>
          </div>
          <FileExplorer @select="selectMedia" />
        </section>

        <!-- Media Player Tab -->
        <section class="h-full" v-show="activeTab === 'player'">
          <MediaPlayer
            v-if="
              store.isReadyToCast && store.selectedDevice && store.selectedMedia
            "
            :device="store.selectedDevice"
            :mediaPath="store.selectedMedia"
            :isLoading="store.isLoading"
            @cast="handleCast"
            @back="activeTab = 'files'"
          />
          <div v-else class="text-center py-12">
            <Play :size="64" class="text-gray-600 mx-auto mb-4" />
            <p class="text-gray-400 text-lg mb-2">No media selected</p>
            <p class="text-gray-500 text-sm mb-6">
              Select a device and media file to start casting
            </p>
            <button @click="activeTab = 'devices'" class="btn-primary">
              Go to Devices
            </button>
          </div>
        </section>
      </main>
    </div>
  </div>
</template>
