import { defineStore } from "pinia";
import { ref } from "vue";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  ScanLibrary,
  OpenLibraryFolderDialog,
  TranslateSeason,
  CancelSeasonTranslation,
  IdentifyLibrary,
  PreviewOrganize,
  OrganizeLibrary,
} from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";
import { useToast } from "vue-toastification";
import { useSettingsStore } from "./settings";

export const useLibraryStore = defineStore("library", () => {
  const toast = useToast();
  const settingsStore = useSettingsStore();

  const scanResult = ref<main.LibraryScanResult | null>(null);
  const isScanning = ref(false);
  const isTranslating = ref(false);
  const translateProgress = ref<main.SeasonTranslateProgress | null>(null);
  const isIdentifying = ref(false);
  const identifyProgress = ref<main.LibraryIdentifyProgress | null>(null);
  // organize state
  const organizePlan = ref<main.OrganizeMove[]>([]);
  const isPreviewing = ref(false);
  const isOrganizing = ref(false);

  EventsOn("library:identify:progress", (p: main.LibraryIdentifyProgress) => {
    identifyProgress.value = p;
    if (p.status === "done") {
      isIdentifying.value = false;
      toast.success("Library identification complete!");
    } else if (p.status === "error") {
      isIdentifying.value = false;
      toast.error(`Identification failed: ${p.message}`);
    }
  });

  EventsOn("library:translate:progress", (p: main.SeasonTranslateProgress) => {
    translateProgress.value = p;
    if (p.status === "done") {
      isTranslating.value = false;
      toast.success(`Season translation complete!`);
    } else if (p.status === "cancelled") {
      isTranslating.value = false;
      toast.info("Season translation cancelled");
    } else if (p.status === "error") {
      isTranslating.value = false;
      toast.error(`Season translation failed: ${p.message}`);
    }
  });

  async function pickAndScan() {
    const dir = await OpenLibraryFolderDialog();
    if (!dir) return;
    await scan(dir);
  }

  async function scan(rootPath: string) {
    isScanning.value = true;
    try {
      scanResult.value = await ScanLibrary(rootPath);
      // Persist the root via settings so it survives restarts.
      if (settingsStore.settings) {
        await settingsStore.saveSettings({
          ...settingsStore.settings,
          libraryRoot: rootPath,
        });
      }
    } catch (err: any) {
      toast.error(`Library scan failed: ${err?.message || err}`);
    } finally {
      isScanning.value = false;
    }
  }

  async function startSeasonTranslation(
    showName: string,
    seasonName: string,
    episodePaths: string[],
    targetLanguage: string
  ) {
    if (isTranslating.value) {
      toast.warning("A season translation is already in progress");
      return;
    }
    isTranslating.value = true;
    translateProgress.value = null;
    try {
      await TranslateSeason(showName, seasonName, episodePaths, targetLanguage);
    } catch (err: any) {
      isTranslating.value = false;
      toast.error(`Failed to start season translation: ${err?.message || err}`);
    }
  }

  async function cancelSeasonTranslation() {
    await CancelSeasonTranslation();
  }

  async function identify() {
    if (!scanResult.value) {
      toast.warning("Scan the library first before identifying.");
      return;
    }
    isIdentifying.value = true;
    identifyProgress.value = null;
    try {
      const enriched = await IdentifyLibrary(scanResult.value);
      scanResult.value = enriched;
    } catch (err: any) {
      isIdentifying.value = false;
      toast.error(`Identification failed: ${err?.message || err}`);
    }
  }

  async function previewOrganize() {
    if (!scanResult.value) {
      toast.warning("Scan and identify the library first.");
      return;
    }
    isPreviewing.value = true;
    try {
      organizePlan.value = await PreviewOrganize(scanResult.value);
    } catch (err: any) {
      toast.error(`Preview failed: ${err?.message || err}`);
    } finally {
      isPreviewing.value = false;
    }
  }

  async function executeOrganize() {
    if (organizePlan.value.length === 0) return;
    isOrganizing.value = true;
    try {
      await OrganizeLibrary(organizePlan.value);
      organizePlan.value = [];
      toast.success("Library organized successfully!");
    } catch (err: any) {
      toast.error(`Organize failed: ${err?.message || err}`);
    } finally {
      isOrganizing.value = false;
    }
  }

  function clearOrganizePlan() {
    organizePlan.value = [];
  }

  return {
    scanResult,
    isScanning,
    isTranslating,
    translateProgress,
    isIdentifying,
    identifyProgress,
    organizePlan,
    isPreviewing,
    isOrganizing,
    pickAndScan,
    scan,
    startSeasonTranslation,
    cancelSeasonTranslation,
    identify,
    previewOrganize,
    executeOrganize,
    clearOrganizePlan,
  };
});
