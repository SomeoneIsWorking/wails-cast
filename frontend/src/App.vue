<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useCastStore } from "./stores/cast";
import DeviceDiscovery from "./components/DeviceDiscovery.vue";
import CastOptions from "./components/CastOptions.vue";
import FileExplorer from "./components/FileExplorer.vue";
import PlaybackControl from "./components/PlaybackControl.vue";
import Settings from "./components/Settings.vue";
import ConfirmModal from "./components/ConfirmModal.vue";
import Footer from "./components/Footer.vue";
import { useConfirm } from "./composables/useConfirm";
import { Tv, Video, Play, Settings as SettingsIcon } from "lucide-vue-next";

const store = useCastStore();
const activeTab = ref<"devices" | "files" | "options">("devices");
const showSettings = ref(false);
const { showConfirmModal, confirmOptions, isConfirmLoading } = useConfirm();

onMounted(() => {
  store.checkFFmpeg();
  store.discoverDevices();
});

const selectDevice = (device: any) => {
  store.selectDevice(device);
  activeTab.value = "files";
};
</script>

<template>
  <div
    class="min-h-screen bg-linear-to-br from-gray-900 via-gray-800 to-gray-900 text-white flex flex-col"
  >
    <div class="container mx-auto px-4 py-6 max-w-6xl flex-1 flex flex-col">
      <!-- Header -->
      <header class="px-2">
        <div class="flex items-center justify-between">
          <div>
            <h1
              class="text-4xl font-bold bg-linear-to-r from-blue-400 to-purple-500 bg-clip-text text-transparent"
            >
              Wails Cast
            </h1>
          </div>
          <button
            @click="showSettings = true"
            class="btn-icon text-gray-400 hover:text-white"
            title="Settings"
          >
            <SettingsIcon :size="24" />
          </button>
        </div>
      </header>

      <!-- Playback Control (shown when something is playing) -->
      <PlaybackControl v-if="store.isCasting" />

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
            v-if="store.trackInfo"
            :class="[
              'px-4 py-2 font-medium transition-all duration-200 border-b-2',
              activeTab === 'options'
                ? 'border-blue-500 text-blue-400'
                : 'border-transparent text-gray-400 hover:text-gray-200',
            ]"
            @click="activeTab = 'options'"
          >
            <span class="flex items-center gap-2">
              <Play :size="18" /> Cast Options
            </span>
          </button>
        </div>

        <!-- Device Discovery Tab -->
        <section class="h-full" v-show="activeTab === 'devices'">
          <DeviceDiscovery @select="selectDevice" />
        </section>

        <!-- Media Files Tab -->
        <section class="h-full" v-show="activeTab === 'files'">
          <FileExplorer @options="activeTab = 'options'" />
        </section>

        <!-- Cast Options Tab -->
        <section class="h-full" v-show="activeTab === 'options'">
          <Suspense>
            <CastOptions />
          </Suspense>
        </section>
      </main>
    </div>

    <!-- Footer -->
    <Footer />

    <!-- Settings Modal -->
    <Settings v-model="showSettings" />

    <!-- Global Confirm Modal -->
    <ConfirmModal
      v-model="showConfirmModal"
      :title="confirmOptions.title"
      :message="confirmOptions.message"
      :confirm-text="confirmOptions.confirmText"
      :cancel-text="confirmOptions.cancelText"
      :variant="confirmOptions.variant"
      :loading="isConfirmLoading"
      @confirm="(confirmOptions as any)._handleConfirm?.()"
      @cancel="(confirmOptions as any)._handleCancel?.()"
    />
  </div>
</template>
