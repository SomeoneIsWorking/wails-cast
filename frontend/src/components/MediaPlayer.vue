<script setup lang="ts">
import { ref, computed } from "vue";
import { useCastStore } from "../stores/cast";
import { mediaService } from "../services/media";
import type { Device } from "../stores/cast";
import "./MediaPlayer.css";
import "./common.css";

interface Props {
  device: Device;
  mediaPath: string;
  isLoading: boolean;
}

defineProps<Props>();

const emit = defineEmits<{
  cast: [];
  back: [];
}>();

const store = useCastStore();
const isCasting = ref(false);
const castResult = ref<string | null>(null);
const mediaURL = ref<string>("");

const fileName = computed(() => store.selectedMedia?.split("/").pop() || "");

const handleCast = async () => {
  isCasting.value = true;
  castResult.value = null;

  try {
    await mediaService.castToDevice(
      store.selectedDevice!.url,
      store.selectedMedia!
    );
    castResult.value = "Cast successful!";
    store.clearError();
  } catch (error: unknown) {
    const errorMsg = error instanceof Error ? error.message : String(error);
    store.setError(errorMsg);
  } finally {
    isCasting.value = false;
  }
};

const generateMediaURL = async () => {
  try {
    const url = await mediaService.getMediaURL(store.selectedMedia!);
    mediaURL.value = url;
  } catch (error: unknown) {
    store.setError("Failed to generate media URL");
  }
};

const copyToClipboard = () => {
  navigator.clipboard.writeText(mediaURL.value);
};

generateMediaURL();
</script>

<template>
  <div class="media-player">
    <div class="player-header">
      <button @click="$emit('back')" class="back-btn">← Back</button>
      <h2>▶️ Cast Media</h2>
      <div style="width: 60px"></div>
    </div>

    <div class="player-content">
      <div class="media-info">
        <div class="media-icon">🎬</div>
        <div class="media-details">
          <h3>{{ fileName }}</h3>
          <p class="media-path">{{ mediaPath }}</p>
        </div>
      </div>

      <div class="device-info">
        <div class="device-icon">📺</div>
        <div class="device-details">
          <h3>{{ device.name }}</h3>
          <p class="device-type">{{ device.type }}</p>
          <p class="device-address">{{ device.address }}</p>
        </div>
      </div>

      <div v-if="mediaURL" class="url-section">
        <h3>Media URL</h3>
        <div class="url-display">
          <code>{{ mediaURL }}</code>
          <button @click="copyToClipboard" class="copy-btn">📋 Copy</button>
        </div>
      </div>

      <div
        v-if="castResult"
        :class="['cast-result', { success: castResult.includes('Casting') }]"
      >
        <p>{{ castResult }}</p>
      </div>
    </div>

    <div class="player-footer">
      <button @click="$emit('back')" class="cancel-btn">Cancel</button>
      <button
        @click="handleCast"
        :disabled="isCasting || isLoading"
        class="cast-btn"
      >
        <span v-if="isCasting || isLoading" class="spinner"></span>
        {{ isCasting || isLoading ? "Casting..." : "Cast Now" }}
      </button>
    </div>
  </div>
</template>
