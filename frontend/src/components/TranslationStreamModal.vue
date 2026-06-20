<script setup lang="ts">
import { ref, computed } from "vue";
import { X, Download, Languages, Square } from "lucide-vue-next";
import {
  ExportEmbeddedSubtitles,
  GenerateTranslationPrompt,
  ProcessPastedTranslation,
} from "../../wailsjs/go/main/App";
import { useToast } from "vue-toastification";
import { useCastStore } from "@/stores/cast";
import { useSettingsStore } from "@/stores/settings";
import { useTranslationStore } from "@/stores/translation";
import { storeToRefs } from "pinia";
import LoadingIcon from "./LoadingIcon.vue";

const castStore = useCastStore();
const settingsStore = useSettingsStore();
const translationStore = useTranslationStore();
const toast = useToast();

const { isTranslating, isCancelling, streamContent } =
  storeToRefs(translationStore);

const showDialog = defineModel<boolean>();
const isExporting = ref(false);
const targetLanguage = ref(
  translationStore.targetLanguage || settingsStore.settings.defaultTranslationLanguage
);
const generatedPrompt = ref("");
const pastedAnswer = ref("");

const trackInfo = computed(() => castStore.trackInfo);

const handleExportSubtitles = async () => {
  if (!trackInfo.value) return;

  isExporting.value = true;
  try {
    await ExportEmbeddedSubtitles(trackInfo.value.Path);
    toast.success("Subtitles exported successfully!");
  } finally {
    isExporting.value = false;
  }
};

const handleTranslateSubtitles = async () => {
  if (!trackInfo.value) return;
  if (!targetLanguage.value.trim()) {
    toast.error("Please enter a target language");
    return;
  }

  try {
    await translationStore.start(trackInfo.value.Path, targetLanguage.value);
    toast.info(`Translation started for ${targetLanguage.value}`);
  } catch (error: any) {
    toast.error(`Failed to start translation: ${error?.message || error}`);
  }
};

const handleCancelTranslation = async () => {
  await translationStore.cancel();
};

const handleGeneratePrompt = async () => {
  if (!trackInfo.value) return;
  try {
    const prompt = await GenerateTranslationPrompt(
      trackInfo.value.Path,
      targetLanguage.value
    );
    generatedPrompt.value = prompt || "";
    toast.info("Prompt generated");
  } catch (err: any) {
    toast.error(`Failed to generate prompt: ${err?.message || err}`);
  }
};

const handleProcessPasted = async () => {
  if (!trackInfo.value) return;
  if (!pastedAnswer.value.trim()) {
    toast.error("Paste the LLM output into the answer area first");
    return;
  }
  try {
    await ProcessPastedTranslation(
      trackInfo.value.Path,
      targetLanguage.value,
      pastedAnswer.value
    );
    toast.success("Pasted answer processed");
  } catch (err: any) {
    toast.error(`Processing failed: ${err?.message || err}`);
  }
};

const handleClose = () => {
  showDialog.value = false;
  // Keep the stream around while translating so reopening shows live progress;
  // only clear it once the run has finished.
  translationStore.clearStream();
};
</script>

<template>
  <div
    v-if="showDialog"
    class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
    @click.self="handleClose"
  >
    <div
      class="bg-gray-800 rounded-md p-6 max-w-4xl w-full mx-4 max-h-[80vh] flex flex-col"
    >
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-2xl font-bold text-white">Subtitle Options</h2>
        <button @click="handleClose" class="btn-close">
          <X class="w-6 h-6" />
        </button>
      </div>

      <!-- Export Section -->
      <div class="mb-4 flex gap-2">
        <button
          @click="handleExportSubtitles"
          :disabled="isExporting"
          class="btn-primary text-sm text-nowrap"
        >
          <Download class="w-4 h-4" />
          {{ isExporting ? "Exporting..." : "Export embedded to WebVTT" }}
        </button>
        <div class="flex-1"></div>
        <template v-if="!isTranslating">
          <input
            v-model="targetLanguage"
            type="text"
            placeholder="Target language (e.g., Turkish)"
            class="bg-gray-700 w-50! text-white rounded-md p-2 text-sm"
          />
          <button
            @click="handleTranslateSubtitles"
            :disabled="!targetLanguage.trim()"
            class="btn-primary text-sm"
          >
            <Languages class="w-4 h-4" />
            Translate
          </button>
        </template>
        <template v-else>
          <div
            class="bg-gray-700 text-white rounded-md p-2 flex px-4 text-sm items-center"
          >
            <LoadingIcon class="w-4 h-4 mr-2" />
            Translating to {{ translationStore.targetLanguage }}...
          </div>
          <button
            @click="handleCancelTranslation"
            :disabled="isCancelling"
            class="btn-danger text-sm"
          >
            <Square class="w-4 h-4" />
            {{ isCancelling ? "Cancelling..." : "Cancel" }}
          </button>
        </template>
      </div>
      <!-- Prompt / Pasted Answer Section -->
      <div class="mb-4 grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <div class="mb-2 text-sm text-white">Generated Prompt</div>
          <textarea
            v-model="generatedPrompt"
            rows="8"
            class="w-full bg-gray-700 text-white rounded-md p-2 text-sm font-mono"
          ></textarea>
          <div class="flex gap-2 mt-2">
            <button @click="handleGeneratePrompt" class="btn-secondary text-sm">
              Generate Prompt
            </button>
            <div class="flex-1"></div>
          </div>
        </div>

        <div>
          <div class="mb-2 text-sm text-white">Paste LLM Answer</div>
          <textarea
            v-model="pastedAnswer"
            rows="8"
            class="w-full bg-gray-700 text-white rounded-md p-2 text-sm font-mono"
            placeholder="Paste the model output here (including optional <llm_output> tags)"
          ></textarea>
          <div class="flex gap-2 mt-2">
            <button @click="handleProcessPasted" class="btn-primary text-sm">
              Process Pasted Answer
            </button>
            <div class="flex-1"></div>
          </div>
        </div>
      </div>
      <!-- Stream Output -->
      <div
        v-if="streamContent"
        class="flex-1 overflow-y-auto bg-gray-900 rounded-md p-4 font-mono text-sm text-green-400"
      >
        <pre class="whitespace-pre-wrap text-left">{{ streamContent }}</pre>
      </div>
    </div>
  </div>
</template>
