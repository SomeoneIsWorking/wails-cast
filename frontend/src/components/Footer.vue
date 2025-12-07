<script setup lang="ts">
import { useCastStore } from "../stores/cast";
import { AlertCircle, RefreshCw } from "lucide-vue-next";

const store = useCastStore();
</script>

<template>
  <footer class="mt-auto py-4 border-t border-gray-800 bg-gray-900">
    <div class="container mx-auto px-4 max-w-6xl">
      <div class="flex items-center justify-between text-xs text-gray-500">
        <div class="flex items-center gap-2">
          <span>© 2025 Wails Cast</span>
        </div>
        
        <!-- FFmpeg Status -->
        <div class="flex items-center gap-3">
          <div v-if="store.ffmpegInfo" class="flex items-center gap-3">
            <!-- FFmpeg -->
            <div v-if="store.ffmpegInfo.ffmpegInstalled" class="flex items-center gap-1.5">
              <div class="w-1.5 h-1.5 bg-green-500 rounded-full"></div>
              <span class="text-gray-400">
                ffmpeg
                <span v-if="store.ffmpegInfo.ffmpegVersion" class="text-gray-500">
                  ({{ store.ffmpegInfo.ffmpegVersion }})
                </span>
              </span>
            </div>
            <div v-else class="flex items-center gap-1.5">
              <AlertCircle class="text-red-400" :size="12" />
              <span class="text-red-400">ffmpeg missing</span>
            </div>

            <!-- FFprobe -->
            <div v-if="store.ffmpegInfo.ffprobeInstalled" class="flex items-center gap-1.5">
              <div class="w-1.5 h-1.5 bg-green-500 rounded-full"></div>
              <span class="text-gray-400">
                ffprobe
                <span v-if="store.ffmpegInfo.ffprobeVersion" class="text-gray-500">
                  ({{ store.ffmpegInfo.ffprobeVersion }})
                </span>
              </span>
            </div>
            <div v-else class="flex items-center gap-1.5">
              <AlertCircle class="text-red-400" :size="12" />
              <span class="text-red-400">ffprobe missing</span>
            </div>

            <!-- Retry button if either is missing -->
            <button
              v-if="!store.ffmpegInfo.ffmpegInstalled || !store.ffmpegInfo.ffprobeInstalled"
              @click="store.checkFFmpeg()"
              class="text-gray-400 hover:text-gray-300 transition-colors"
              title="Recheck FFmpeg installation"
            >
              <RefreshCw :size="12" />
            </button>

            <!-- Installation link -->
            <a
              v-if="!store.ffmpegInfo.ffmpegInstalled || !store.ffmpegInfo.ffprobeInstalled"
              href="https://ffmpeg.org/download.html"
              target="_blank"
              rel="noopener noreferrer"
              class="text-blue-400 hover:text-blue-300 underline transition-colors"
            >
              Install Guide
            </a>
          </div>
        </div>
      </div>
    </div>
  </footer>
</template>
