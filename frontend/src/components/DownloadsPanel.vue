<template>
  <div
    class="fixed inset-0 z-40"
    :class="{ 'pointer-events-none': !modelValue }"
    aria-hidden="false"
  >
    <transition name="backdrop">
      <div
        v-if="modelValue"
        class="absolute inset-0 bg-black/50 dark:bg-black/60 pointer-events-auto"
        @click="modelValue = false"
      ></div>
    </transition>

    <transition name="slide">
      <aside
        v-if="modelValue"
        class="absolute right-0 top-0 bottom-0 w-96 bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 shadow-lg p-4 overflow-auto pointer-events-auto"
      >
        <h3 class="text-lg font-semibold mb-4">Downloads</h3>

        <div v-if="entries.length === 0" class="text-sm text-gray-500">
          No downloads
        </div>

        <ul>
          <li
            v-for="item in entries"
            :key="item.URL + '|' + item.MediaType + '|' + item.Track"
            class="mb-4 border-b pb-2 flex flex-col gap-2"
          >
            <div class="flex-1">
              <div class="text-sm font-medium">{{ item.URL }}</div>
              <div class="text-xs text-gray-500">
                {{ item.MediaType }} • Track {{ item.Track }}
              </div>
            </div>

            <ProgressBar
              :progress="item.Segments.filter((v) => v).length"
              :total="item.Segments.length"
              class="flex-1"
            />
            
            <div class="flex gap-2">
              <button
                v-if="item.Status === 'STOPPED'"
                @click.stop="start(item)"
                class="btn-primary"
              >
                <Play></Play>
              </button>
              <button
                v-if="item.Status === 'INPROGRESS'"
                @click.stop="stop(item)"
                class="btn-primary"
              >
                <Square></Square>
              </button>
              <button
                v-if="item.Status !== 'INPROGRESS'"
                @click.stop="item.Status = 'IDLE'"
                class="btn-secondary text-sm"
              >
                <Trash></Trash>
              </button>
            </div>
          </li>
        </ul>
      </aside>
    </transition>
  </div>
</template>

<script lang="ts" setup>
import { computed } from "vue";
import { useDownloadsStore } from "@/stores/downloads";
import ProgressBar from "./ProgressBar.vue";
import { Play, Square, Trash } from "lucide-vue-next";

const store = useDownloadsStore();
const modelValue = defineModel<boolean>();

const entries = computed(() =>
  Object.values(store.downloads).filter(
    (d) => d.Status !== "COMPLETED" && d.Status !== "IDLE"
  )
);

const start = (item: any) => {
  store.startDownload(item.Url, item.MediaType, item.Track);
};

const stop = (item: any) => {
  store.stopDownload(item.Url, item.MediaType, item.Track);
};
</script>

<style scoped>
.backdrop-enter-active,
.backdrop-leave-active {
  transition: opacity 200ms ease;
}
.backdrop-enter-from,
.backdrop-leave-to {
  opacity: 0;
}
.backdrop-enter-to,
.backdrop-leave-from {
  opacity: 1;
}

.slide-enter-active,
.slide-leave-active {
  transition: transform 250ms ease;
}
.slide-enter-from,
.slide-leave-to {
  transform: translateX(100%);
}
.slide-enter-to,
.slide-leave-from {
  transform: translateX(0);
}
</style>
