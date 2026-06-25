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

        <!-- Torrents (from the active remote source's qBittorrent) -->
        <div v-if="torrents.length" class="mb-5">
          <div class="text-xs uppercase tracking-wider text-gray-500 mb-2">
            Torrents — {{ libraryStore.browseSource.name }}
          </div>
          <div
            v-for="t in torrents"
            :key="t.hash"
            class="mb-3 bg-gray-100 dark:bg-slate-900/60 rounded-lg p-2"
          >
            <div class="flex items-center justify-between gap-2 mb-1">
              <span class="text-sm truncate">{{ t.name }}</span>
              <span class="text-xs text-gray-500 whitespace-nowrap">
                {{ (t.progress * 100).toFixed(1) }}%
              </span>
            </div>
            <div class="h-1.5 rounded-full bg-gray-300 dark:bg-slate-700 overflow-hidden">
              <div
                class="h-full bg-gradient-to-r from-blue-500 to-purple-500"
                :style="{ width: `${Math.min(100, t.progress * 100)}%` }"
              />
            </div>
            <div class="flex items-center gap-3 mt-1 text-xs text-gray-500">
              <span class="capitalize">{{ t.state }}</span>
              <span>↓ {{ fmtBytes(t.dlspeed) }}/s</span>
              <span class="ml-auto">{{ fmtBytes(t.size) }}</span>
            </div>
          </div>
        </div>

        <div
          v-if="entries.length === 0 && torrents.length === 0"
          class="text-sm text-gray-500"
        >
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
import { useLibraryStore } from "@/stores/library";
import ProgressBar from "./ProgressBar.vue";
import { Play, Square, Trash } from "lucide-vue-next";

const store = useDownloadsStore();
const libraryStore = useLibraryStore();
const modelValue = defineModel<boolean>();

const entries = computed(() =>
  Object.values(store.downloads).filter(
    (d) => d.Status !== "COMPLETED" && d.Status !== "IDLE"
  )
);

const torrents = computed(() => libraryStore.torrents);

function fmtBytes(n: number): string {
  if (!n || n < 0) return "0 B";
  const u = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v < 10 && i > 0 ? 1 : 0)} ${u[i]}`;
}

const start = (item: any) => {
  store.startDownload(item.URL, item.MediaType, item.Track);
};

const stop = (item: any) => {
  store.stopDownload(item.URL, item.MediaType, item.Track);
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
