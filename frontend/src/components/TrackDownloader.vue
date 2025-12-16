<template>
  <template v-if="isRemote">
    <div>
      <div class="flex gap-2">
        <slot></slot>
        <button
          @click="start"
          :disabled="loading"
          v-if="downloadState?.Status !== 'INPROGRESS'"
          class="btn-secondary px-3"
        >
          <Download class="w-4 h-4" />
        </button>
        <button
          v-else
          @click="stop"
          :disabled="loading"
          class="btn-secondary px-3"
        >
          <StopCircle class="w-4 h-4" />
        </button>
      </div>
      <ProgressBar
        class="mt-2"
        v-if="isRemote"
        :proress="downloadState?.Downloaded || 0"
        :total="downloadState?.Total || 0"
      />
    </div>
  </template>
  <template v-else>
    <slot></slot>
  </template>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useCastStore } from "../stores/cast";
import { useDownloadsStore } from "../stores/downloads";
import { Download, StopCircle } from "lucide-vue-next";
import ProgressBar from "./ProgressBar.vue";
const castStore = useCastStore();
const downloadsStore = useDownloadsStore();

const props = defineProps<{
  path: string;
  type: "video" | "audio";
}>();

const loading = ref(false);

const start = () => {
  loading.value = true;
  try {
    downloadsStore.startDownload(
      props.path,
      props.type,
      props.type === "video"
        ? castStore.castOptions!.VideoTrack
        : castStore.castOptions!.AudioTrack
    );
  } finally {
    loading.value = false;
  }
};

const stop = () => {
  loading.value = true;
  try {
    downloadsStore.stopDownload(
      props.path,
      props.type,
      props.type === "video"
        ? castStore.castOptions!.VideoTrack
        : castStore.castOptions!.AudioTrack
    );
  } finally {
    loading.value = false;
  }
};

const downloadState = computed(() => {
  return downloadsStore.getDownloadState(
    props.path,
    props.type,
    props.type === "video"
      ? castStore.castOptions!.VideoTrack
      : castStore.castOptions!.AudioTrack
  );
});

const isRemote = computed(() => {
  return props.path.startsWith("http://") || props.path.startsWith("https://");
});

const loadTrackProgress = async () => {
  const trackIndex =
    props.type === "video"
      ? castStore.castOptions!.VideoTrack
      : castStore.castOptions!.AudioTrack;
  await downloadsStore.loadTrackProgress(props.path, props.type, trackIndex);
};

onMounted(() => {
  if (isRemote.value) {
    loadTrackProgress();
  }
});
</script>
