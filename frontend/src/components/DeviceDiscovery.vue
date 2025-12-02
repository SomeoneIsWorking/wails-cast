<script setup lang="ts">
import { Device } from "@/services/device";
import { useCastStore } from "../stores/cast";
import { RefreshCw, Cast, Check, Loader2, Network } from "lucide-vue-next";
import { main } from "../../wailsjs/go/models";

const emit = defineEmits<{
  select: [device: Device];
  discover: [];
}>();

const localDevice = {
  address: "local",
  name: "Local Device",
  type: "Local",
  url: "local",
  host: "local",
} as main.Device;

const store = useCastStore();

const selectDevice = (device: Device) => {
  store.selectDevice(device);
  emit("select", device);
};
</script>

<template>
  <div class="device-discovery h-full flex flex-col">
    <div class="flex items-center justify-between mb-4">
      <div></div>
      <button
        @click="store.discoverDevices"
        :disabled="store.isLoading"
        class="btn-primary flex items-center gap-2"
      >
        <RefreshCw :size="18" :class="{ 'animate-spin': store.isLoading }" />
        {{ store.isLoading ? "Searching..." : "Scan Network" }}
      </button>
    </div>

    <div class="flex-1 overflow-auto space-y-4">
      <!-- Device List -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div
          v-for="device in [localDevice, ...store.devices]"
          :key="device.url"
          @click="selectDevice(device)"
          :class="[
            'device-item',
            {
              'device-item-selected': store.selectedDevice?.url === device.url,
            },
          ]"
        >
          <div class="flex items-center gap-3">
            <div class="p-3 bg-blue-600 rounded-lg">
              <Network :size="24" v-if="device.address == 'local'"></Network>
              <Cast :size="24" v-else />
            </div>
            <div class="flex-1 min-w-0">
              <h3 class="font-semibold text-lg truncate">{{ device.name }}</h3>
              <p class="text-sm text-gray-400 truncate" v-if="device.address !== 'local'" >{{ device.type }}</p>
              <p class="text-xs text-gray-500 truncate" v-if="device.address !== 'local'">{{ device.address }}</p>
            </div>
          </div>
          <div v-if="store.selectedDevice?.url === device.url" class="shrink-0">
            <Check :size="24" class="text-blue-400" />
          </div>
        </div>
      </div>

      <!-- Device Count -->
      <div
        v-if="store.hasDevices && !store.isLoading"
        class="text-center text-sm text-gray-500"
      >
        Found {{ store.devices.length }} device{{
          store.devices.length > 1 ? "s" : ""
        }}
      </div>

      <!-- Loading State -->
      <div
        v-if="store.isLoading"
        class="flex flex-col items-center justify-center py-12"
      >
        <Loader2 :size="48" class="text-blue-400 mb-4 animate-spin" />
        <p class="text-gray-400">Discovering devices...</p>
      </div>

      <!-- Empty State -->
      <div
        v-else-if="!store.hasDevices"
        class="flex flex-col items-center justify-center py-12"
      >
        <Cast :size="64" class="text-gray-600 mb-4" />
        <p class="text-gray-400 text-lg mb-2">No devices found</p>
        <p class="text-gray-500 text-sm mb-6">
          Make sure your Chromecast is on the same network
        </p>
        <button @click="store.discoverDevices" class="btn-primary">
          Search for Devices
        </button>
      </div>
    </div>
  </div>
</template>
