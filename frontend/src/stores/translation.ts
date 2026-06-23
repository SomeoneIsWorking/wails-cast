import { defineStore } from "pinia";
import { ref } from "vue";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  TranslateExportedSubtitles,
  CancelTranslation,
} from "../../wailsjs/go/main/App";
import { useToast } from "vue-toastification";
import { useCastStore } from "./cast";

export const useTranslationStore = defineStore("translation", () => {
  const isTranslating = ref(false);
  const isCancelling = ref(false);
  const streamContent = ref("");
  const targetLanguage = ref("");
  const translatedFiles = ref<string[]>([]);
  // Path of the file currently being translated (single-file translation), so
  // UI can show the "Translating…" state only where it actually applies rather
  // than globally. Only one single-file translation runs at a time.
  const activePath = ref<string | null>(null);

  const toast = useToast();

  // Listeners are registered once for the app lifetime so the stream keeps
  // accumulating even while the modal is closed.
  EventsOn("translation:stream", (chunk: string) => {
    streamContent.value += chunk;
  });

  EventsOn("translation:complete", (files: string[]) => {
    isTranslating.value = false;
    isCancelling.value = false;
    activePath.value = null;
    translatedFiles.value = files || [];
    if (files && files.length > 0) {
      const castStore = useCastStore();
      if (castStore.castOptions) {
        castStore.castOptions.SubtitleType = "external";
        castStore.castOptions.SubtitlePath = files[0];
      }
      toast.success("Translated subtitle(s) completed!");
    }
  });

  EventsOn("translation:error", (error: string) => {
    isTranslating.value = false;
    isCancelling.value = false;
    activePath.value = null;
    toast.error(`Translation failed: ${error}`);
  });

  EventsOn("translation:cancelled", () => {
    isTranslating.value = false;
    isCancelling.value = false;
    activePath.value = null;
    toast.info("Translation cancelled");
  });

  async function start(path: string, language: string) {
    streamContent.value = "";
    translatedFiles.value = [];
    targetLanguage.value = language;
    isTranslating.value = true;
    isCancelling.value = false;
    activePath.value = path;
    try {
      await TranslateExportedSubtitles(path, language);
    } catch (e) {
      isTranslating.value = false;
      activePath.value = null;
      throw e;
    }
  }

  async function cancel() {
    if (!isTranslating.value) return;
    isCancelling.value = true;
    await CancelTranslation();
  }

  function clearStream() {
    if (!isTranslating.value) {
      streamContent.value = "";
    }
  }

  // resetStream clears the buffer unconditionally. Used by the library's season
  // translation to show only the current episode's stream (a season run feeds
  // translation:stream continuously across episodes).
  function resetStream() {
    streamContent.value = "";
  }

  return {
    isTranslating,
    isCancelling,
    streamContent,
    targetLanguage,
    translatedFiles,
    activePath,
    start,
    cancel,
    clearStream,
    resetStream,
  };
});
