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
  DiscoverCastInstances,
  RemoteLibraryTree,
  RemoteIdentify,
  RemoteOrganizePreview,
  RemoteOrganizeExecute,
  RemoteTranslateSeason,
  RemoteSeasonStatus,
  RemoteSeasonCancel,
  RemoteTorrents,
  RemoteAddTorrent,
} from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";
import { useToast } from "vue-toastification";
import { useSettingsStore } from "./settings";
import { useTranslationStore } from "./translation";
import { LOCAL_SOURCE, type Source } from "@/services/source";

// These are event payloads (emitted via wails events, not bound-method return
// types), so wails does not generate them in models.ts — define them locally so
// they survive binding regeneration.
export interface SeasonTranslateProgress {
  showName: string;
  seasonName: string;
  targetLanguage: string;
  totalEpisodes: number;
  currentEpisode: number;
  status: string;
  message: string;
}

export interface LibraryIdentifyProgress {
  total: number;
  current: number;
  showName: string;
  status: string;
  message: string;
}

export const useLibraryStore = defineStore("library", () => {
  const toast = useToast();
  const settingsStore = useSettingsStore();

  const scanResult = ref<main.LibraryScanResult | null>(null);
  const isScanning = ref(false);
  const isTranslating = ref(false);
  const translateProgress = ref<SeasonTranslateProgress | null>(null);
  const isIdentifying = ref(false);
  const identifyProgress = ref<LibraryIdentifyProgress | null>(null);
  // organize state
  const organizePlan = ref<main.OrganizeMove[]>([]);
  const isPreviewing = ref(false);
  const isOrganizing = ref(false);

  // Sources: "This Mac" plus any discovered remote instances. browseSource is
  // the source whose library is currently shown; the same tree UI renders all.
  const sources = ref<Source[]>([LOCAL_SOURCE]);
  const browseSource = ref<Source>(LOCAL_SOURCE);
  const isDiscoveringSources = ref(false);

  const isRemoteBrowse = () => browseSource.value.kind === "remote";

  EventsOn("library:identify:progress", (p: LibraryIdentifyProgress) => {
    identifyProgress.value = p;
    if (p.status === "done") {
      isIdentifying.value = false;
      toast.success("Library identification complete!");
    } else if (p.status === "error") {
      isIdentifying.value = false;
      toast.error(`Identification failed: ${p.message}`);
    }
  });

  // Tracks the last in-progress episode index so we can reset the live stream
  // buffer whenever the season translation advances to a new episode.
  let lastTranslateEpisode = 0;

  EventsOn("library:translate:progress", (p: SeasonTranslateProgress) => {
    // Only honour local events when browsing the local source (remote progress
    // is driven by polling instead).
    if (isRemoteBrowse()) return;
    applySeasonProgress(p);
  });

  function applySeasonProgress(p: SeasonTranslateProgress) {
    translateProgress.value = p;
    if (p.currentEpisode && p.currentEpisode !== lastTranslateEpisode) {
      lastTranslateEpisode = p.currentEpisode;
      useTranslationStore().resetStream();
    }
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
  }

  // ─── Sources ────────────────────────────────────────────────────────────────

  async function discoverSources() {
    isDiscoveringSources.value = true;
    try {
      const instances = await DiscoverCastInstances();
      const remotes: Source[] = instances.map((i) => ({
        kind: "remote" as const,
        name: i.name,
        base: i.url,
        token: "",
      }));
      sources.value = [LOCAL_SOURCE, ...remotes];
      // If the current browse source vanished, fall back to local.
      if (
        browseSource.value.kind === "remote" &&
        !remotes.find((r) => r.base === browseSource.value.base)
      ) {
        await selectSource(LOCAL_SOURCE);
      }
    } catch (err: any) {
      toast.error(`Discovery failed: ${err?.message || err}`);
    } finally {
      isDiscoveringSources.value = false;
    }
  }

  async function selectSource(source: Source) {
    browseSource.value = source;
    startTorrentPoll();
    await rescan();
  }

  // ─── Torrents (per remote source) ─────────────────────────────────────────────

  const torrents = ref<main.TorrentStatus[]>([]);
  const magnet = ref("");
  const magnetBusy = ref(false);
  let torrentPoll: number | null = null;
  let lastTorrentSig = "";
  let pollTick = 0;

  function stopTorrentPoll() {
    if (torrentPoll !== null) {
      clearInterval(torrentPoll);
      torrentPoll = null;
    }
  }
  async function refreshTorrents() {
    if (!isRemoteBrowse()) {
      torrents.value = [];
      lastTorrentSig = "";
      return;
    }
    try {
      torrents.value = await RemoteTorrents(
        browseSource.value.base,
        browseSource.value.token
      );
    } catch {
      return; // transient
    }

    // The tree must follow the torrents: a magnet adds files to the library that
    // /library/tree only reveals on a re-fetch. Refresh the tree when the set of
    // torrents changes (added / completed), and periodically while any are still
    // downloading (so partial episodes appear as they land).
    const sig = torrents.value
      .map((t) => `${t.hash}:${t.progress >= 1 ? "done" : "dl"}`)
      .sort()
      .join(",");
    const downloading = torrents.value.some((t) => t.progress < 1);
    pollTick++;
    if (sig !== lastTorrentSig || (downloading && pollTick % 6 === 0)) {
      lastTorrentSig = sig;
      refreshTreeSilently();
    }
  }
  function startTorrentPoll() {
    stopTorrentPoll();
    refreshTorrents();
    if (isRemoteBrowse()) {
      torrentPoll = window.setInterval(refreshTorrents, 2500);
    }
  }
  async function sendMagnet() {
    const m = magnet.value.trim();
    if (!m || !isRemoteBrowse()) return;
    magnetBusy.value = true;
    try {
      await RemoteAddTorrent(browseSource.value.base, browseSource.value.token, m);
      magnet.value = "";
      toast.success("Sent to qBittorrent");
      await refreshTorrents();
    } catch (err: any) {
      toast.error(`Failed to add magnet: ${err?.message || err}`);
    } finally {
      magnetBusy.value = false;
    }
  }
  // torrentForPath finds the in-progress torrent that owns a library episode
  // path, so the tree can show per-episode download progress.
  function torrentForPath(path: string): main.TorrentStatus | null {
    if (!path) return null;
    for (const t of torrents.value) {
      if (t.progress >= 1) continue;
      const cp = t.content_path || "";
      if (cp && (path === cp || path.startsWith(cp.replace(/\/?$/, "/")))) {
        return t;
      }
    }
    return null;
  }

  // loadTreeResult fetches the tree for the active browse source (no UI side
  // effects), used by both the spinner-showing rescan and the silent refresh.
  async function loadTreeResult(): Promise<main.LibraryScanResult | null> {
    if (isRemoteBrowse()) {
      return await RemoteLibraryTree(
        browseSource.value.base,
        browseSource.value.token
      );
    }
    const root = settingsStore.settings?.libraryRoot;
    return root ? await ScanLibrary(root) : null;
  }

  // rescan loads the tree for the active browse source (shows the spinner).
  async function rescan() {
    isScanning.value = true;
    try {
      scanResult.value = await loadTreeResult();
    } catch (err: any) {
      toast.error(`Library scan failed: ${err?.message || err}`);
    } finally {
      isScanning.value = false;
    }
  }

  // refreshTreeSilently updates the tree in the background (no spinner) so new
  // episodes from in-progress torrent downloads appear without disrupting the UI.
  async function refreshTreeSilently() {
    try {
      const r = await loadTreeResult();
      if (r) scanResult.value = r;
    } catch {
      /* transient */
    }
  }

  // ─── Local-only folder pick ───────────────────────────────────────────────────

  async function pickAndScan() {
    const dir = await OpenLibraryFolderDialog();
    if (!dir) return;
    await scan(dir);
  }

  async function scan(rootPath: string) {
    isScanning.value = true;
    try {
      scanResult.value = await ScanLibrary(rootPath);
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

  // ─── Season translation (local events vs remote polling) ──────────────────────

  let seasonPoll: number | null = null;
  function stopSeasonPoll() {
    if (seasonPoll !== null) {
      clearInterval(seasonPoll);
      seasonPoll = null;
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
    lastTranslateEpisode = 0;
    useTranslationStore().resetStream();
    try {
      if (isRemoteBrowse()) {
        const { base, token } = browseSource.value;
        await RemoteTranslateSeason(
          base,
          token,
          showName,
          seasonName,
          episodePaths,
          targetLanguage
        );
        // Drive progress by polling the remote season status.
        stopSeasonPoll();
        seasonPoll = window.setInterval(async () => {
          try {
            const st = await RemoteSeasonStatus(base, token);
            applySeasonProgress(st as SeasonTranslateProgress);
            if (st.status === "done" || st.status === "cancelled" || st.status === "error") {
              stopSeasonPoll();
            }
          } catch {
            /* transient */
          }
        }, 1500);
      } else {
        await TranslateSeason(showName, seasonName, episodePaths, targetLanguage);
      }
    } catch (err: any) {
      isTranslating.value = false;
      toast.error(`Failed to start season translation: ${err?.message || err}`);
    }
  }

  async function cancelSeasonTranslation() {
    if (isRemoteBrowse()) {
      await RemoteSeasonCancel(browseSource.value.base, browseSource.value.token);
      stopSeasonPoll();
    } else {
      await CancelSeasonTranslation();
    }
  }

  // ─── Identify / Organize (route by source) ────────────────────────────────────

  async function identify() {
    if (!scanResult.value) {
      toast.warning("Scan the library first before identifying.");
      return;
    }
    isIdentifying.value = true;
    identifyProgress.value = null;
    try {
      if (isRemoteBrowse()) {
        // Remote identify is synchronous (no progress events reach us).
        scanResult.value = await RemoteIdentify(
          browseSource.value.base,
          browseSource.value.token,
          scanResult.value
        );
        isIdentifying.value = false;
        toast.success("Library identification complete!");
      } else {
        scanResult.value = await IdentifyLibrary(scanResult.value);
      }
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
      organizePlan.value = isRemoteBrowse()
        ? await RemoteOrganizePreview(
            browseSource.value.base,
            browseSource.value.token,
            scanResult.value
          )
        : await PreviewOrganize(scanResult.value);
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
      if (isRemoteBrowse()) {
        await RemoteOrganizeExecute(
          browseSource.value.base,
          browseSource.value.token,
          organizePlan.value
        );
      } else {
        await OrganizeLibrary(organizePlan.value);
      }
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
    sources,
    browseSource,
    isDiscoveringSources,
    discoverSources,
    selectSource,
    rescan,
    torrents,
    magnet,
    magnetBusy,
    sendMagnet,
    refreshTorrents,
    torrentForPath,
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
