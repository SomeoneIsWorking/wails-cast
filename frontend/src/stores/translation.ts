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

  const toast = useToast();

  // Listeners are registered once for the app lifetime so the stream keeps
  // accumulating even while the modal is closed.
  EventsOn("translation:stream", (chunk: string) => {
    streamContent.value += chunk;
  });

  EventsOn("translation:complete", (files: string[]) => {
    isTranslating.value = false;
    isCancelling.value = false;
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
    toast.error(`Translation failed: ${error}`);
  });

  EventsOn("translation:cancelled", () => {
    isTranslating.value = false;
    isCancelling.value = false;
    toast.info("Translation cancelled");
  });

  async function start(path: string, language: string) {
    streamContent.value = "";
    translatedFiles.value = [];
    targetLanguage.value = language;
    isTranslating.value = true;
    isCancelling.value = false;
    try {
      await TranslateExportedSubtitles(path, language);
    } catch (e) {
      isTranslating.value = false;
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

  return {
    isTranslating,
    isCancelling,
    streamContent,
    targetLanguage,
    translatedFiles,
    start,
    cancel,
    clearStream,
  };
});
