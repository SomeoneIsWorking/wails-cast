<script setup lang="ts">
import { ref, computed, onMounted, watch } from "vue";
import {
  FolderOpen,
  RefreshCw,
  Play,
  Languages,
  Square,
  ChevronDown,
  ChevronRight,
  Library,
  CheckCircle,
  Search,
  FolderCheck,
  Eye,
  X,
  Radio,
  Monitor,
  Magnet,
} from "lucide-vue-next";
import { useLibraryStore } from "@/stores/library";
import { useCastStore } from "@/stores/cast";
import { useSettingsStore } from "@/stores/settings";
import { useTranslationStore } from "@/stores/translation";
import LoadingIcon from "./LoadingIcon.vue";
import { useToast } from "vue-toastification";
import { main } from "../../wailsjs/go/models";

const emit = defineEmits<{
  options: [];
}>();

const libraryStore = useLibraryStore();
const castStore = useCastStore();
const settingsStore = useSettingsStore();
const translationStore = useTranslationStore();
const toast = useToast();

// Which show/season nodes are expanded.
const expandedShows = ref<Set<string>>(new Set());
const expandedSeasons = ref<Set<string>>(new Set());

// Translate modal state. A target is either a whole season (episode undefined)
// or a single episode.
const showTranslateModal = ref(false);
const translateTarget = ref<{
  show: main.LibraryShow;
  season: main.LibrarySeason;
  episode?: main.LibraryEpisode;
} | null>(null);
const translateLanguage = ref(settingsStore.settings?.defaultTranslationLanguage || "English");


// Live translation-stream modal (shared by single-episode and season runs).
const showStreamModal = ref(false);

// Per-episode loading state when clicking Play.
const loadingEpisode = ref<string | null>(null);

// Organize modal — shows the preview list before confirming.
const showOrganizeModal = ref(false);

// ─── Lifecycle ───────────────────────────────────────────────────────────────

onMounted(async () => {
  // Discover remote sources in the background, and load the active source's tree.
  libraryStore.discoverSources();
  if (!libraryStore.scanResult) {
    await libraryStore.rescan();
  }
});

const isRemoteBrowse = computed(() => libraryStore.browseSource.kind === "remote");

// ─── Computed ────────────────────────────────────────────────────────────────

const shows = computed(() => libraryStore.scanResult?.shows ?? []);
const rootPath = computed(() => libraryStore.scanResult?.rootPath ?? "");
const progress = computed(() => libraryStore.translateProgress);
const identifyProgress = computed(() => libraryStore.identifyProgress);
const organizePlan = computed(() => libraryStore.organizePlan);

// ─── Tree helpers ─────────────────────────────────────────────────────────────

function showKey(show: main.LibraryShow) {
  return show.path;
}
function seasonKey(show: main.LibraryShow, season: main.LibrarySeason) {
  return show.path + "|" + season.name;
}

function toggleShow(show: main.LibraryShow) {
  const k = showKey(show);
  if (expandedShows.value.has(k)) {
    expandedShows.value.delete(k);
  } else {
    expandedShows.value.add(k);
  }
}
function toggleSeason(show: main.LibraryShow, season: main.LibrarySeason) {
  const k = seasonKey(show, season);
  if (expandedSeasons.value.has(k)) {
    expandedSeasons.value.delete(k);
  } else {
    expandedSeasons.value.add(k);
  }
}

function isShowExpanded(show: main.LibraryShow) {
  return expandedShows.value.has(showKey(show));
}
function isSeasonExpanded(show: main.LibraryShow, season: main.LibrarySeason) {
  return expandedSeasons.value.has(seasonKey(show, season));
}

// ─── Actions ──────────────────────────────────────────────────────────────────

async function playEpisode(ep: main.LibraryEpisode) {
  // Local playback needs a device picked in the Devices tab; remote playback
  // picks a target from the remote's own devices in Cast Options.
  if (!isRemoteBrowse.value && !castStore.selectedDevice) {
    toast.warning("No device selected. Go to the Devices tab first.");
    return;
  }
  loadingEpisode.value = ep.path;
  try {
    await castStore.prepareEpisode(ep.path, libraryStore.browseSource);
    emit("options");
  } catch (err: any) {
    toast.error(`Failed to load track info: ${err?.message || err}`);
  } finally {
    loadingEpisode.value = null;
  }
}

// Is any translation (single-episode or season) currently running?
const anyTranslating = computed(
  () => libraryStore.isTranslating || translationStore.isTranslating
);

function openTranslateModal(
  show: main.LibraryShow,
  season: main.LibrarySeason,
  episode?: main.LibraryEpisode
) {
  translateTarget.value = { show, season, episode };
  translateLanguage.value =
    settingsStore.settings?.defaultTranslationLanguage || "English";
  showTranslateModal.value = true;
}

async function startTranslate() {
  if (!translateTarget.value) return;
  const { show, season, episode } = translateTarget.value;
  const lang = translateLanguage.value;
  showTranslateModal.value = false;

  if (episode) {
    // Single-episode translation via the shared translation store.
    try {
      await translationStore.start(episode.path, lang, libraryStore.browseSource);
    } catch (err: any) {
      toast.error(`Failed to start translation: ${err?.message || err}`);
    }
    return;
  }

  // Season: only translate episodes that aren't already translated.
  const paths = season.episodes.filter((e) => !e.translated).map((e) => e.path);
  if (paths.length === 0) {
    toast.info("All episodes in this season are already translated.");
    return;
  }
  await libraryStore.startSeasonTranslation(show.name, season.name, paths, lang);
}

async function cancelSeasonTranslate() {
  await libraryStore.cancelSeasonTranslation();
}

// When a single-file translation finishes (activePath clears), re-scan so the
// new subtitle is reflected (translated flag / green check).
watch(
  () => translationStore.activePath,
  (now, prev) => {
    if (prev && !now && rootPath.value) {
      libraryStore.rescan();
    }
  }
);

// Likewise re-scan when a season batch translation finishes.
watch(
  () => libraryStore.isTranslating,
  (now, prev) => {
    if (prev && !now && rootPath.value) {
      libraryStore.rescan();
    }
  }
);

// Cancel whichever translation is active (from the live modal).
async function cancelActiveTranslation() {
  if (libraryStore.isTranslating) {
    await libraryStore.cancelSeasonTranslation();
  } else {
    await translationStore.cancel();
  }
}

function episodeCount(show: main.LibraryShow) {
  return show.seasons.reduce((sum, s) => sum + s.episodes.length, 0);
}

function translatedCount(season: main.LibrarySeason) {
  return season.episodes.filter((e) => e.translated).length;
}

async function runIdentify() {
  await libraryStore.identify();
}

async function openOrganizePreview() {
  await libraryStore.previewOrganize();
  if (libraryStore.organizePlan.length > 0) {
    showOrganizeModal.value = true;
  } else {
    toast.info("Nothing to organize — either no identified episodes or all files are already in the canonical layout.");
  }
}

async function confirmOrganize() {
  showOrganizeModal.value = false;
  await libraryStore.executeOrganize();
  // Re-scan to reflect moved files.
  await libraryStore.rescan();
}

function cancelOrganize() {
  libraryStore.clearOrganizePlan();
  showOrganizeModal.value = false;
}
</script>

<template>
  <div class="library-view h-full flex flex-col">

    <!-- Source bar: This Mac + discovered remotes -->
    <div class="flex items-center gap-2 mb-3 flex-wrap">
      <span class="text-xs text-gray-500 uppercase tracking-wider mr-1">Source</span>
      <button
        v-for="src in libraryStore.sources"
        :key="src.kind === 'local' ? 'local' : src.base"
        @click="libraryStore.selectSource(src)"
        class="text-xs px-3 py-1.5 rounded-full border transition-colors"
        :class="
          (libraryStore.browseSource.base === src.base && libraryStore.browseSource.kind === src.kind)
            ? 'bg-blue-600/40 border-blue-500 text-white'
            : 'bg-gray-800 border-gray-700 text-gray-300 hover:bg-gray-700'
        "
        :title="src.kind === 'remote' ? src.base : 'Local library'"
      >
        <component :is="src.kind === 'remote' ? Radio : Monitor" class="w-3.5 h-3.5 inline mr-1 -mt-0.5" />
        {{ src.name }}
      </button>
      <button
        @click="libraryStore.discoverSources()"
        class="text-xs p-1.5 rounded-full bg-gray-800 border border-gray-700 hover:bg-gray-700"
        :disabled="libraryStore.isDiscoveringSources"
        title="Rediscover remote instances"
      >
        <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': libraryStore.isDiscoveringSources }" />
      </button>
    </div>

    <!-- Toolbar -->
    <div class="flex items-center gap-3 mb-4 flex-wrap">
      <button
        v-if="!isRemoteBrowse"
        @click="libraryStore.pickAndScan()"
        class="btn-primary"
        :disabled="libraryStore.isScanning"
      >
        <FolderOpen class="w-4 h-4" />
        {{ rootPath ? "Change Folder" : "Select Library Folder" }}
      </button>
      <button
        v-if="rootPath"
        @click="libraryStore.rescan()"
        class="btn-secondary"
        :disabled="libraryStore.isScanning"
        title="Rescan"
      >
        <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': libraryStore.isScanning }" />
        Rescan
      </button>
      <button
        v-if="shows.length > 0"
        @click="runIdentify()"
        class="btn-secondary"
        :disabled="libraryStore.isIdentifying"
        title="Look up shows on TMDB to get official episode names"
      >
        <LoadingIcon v-if="libraryStore.isIdentifying" class="w-4 h-4" />
        <Search v-else class="w-4 h-4" />
        Identify
      </button>
      <button
        v-if="shows.length > 0"
        @click="openOrganizePreview()"
        class="btn-secondary"
        :disabled="libraryStore.isPreviewing || libraryStore.isOrganizing"
        title="Preview and apply canonical folder layout (requires identified episodes)"
      >
        <LoadingIcon v-if="libraryStore.isPreviewing" class="w-4 h-4" />
        <FolderCheck v-else class="w-4 h-4" />
        Organize
      </button>
      <span v-if="rootPath" class="text-gray-400 text-sm truncate max-w-xs">{{ rootPath }}</span>
    </div>

    <!-- Magnet input (remote sources send to that host's qBittorrent) -->
    <div v-if="isRemoteBrowse" class="flex items-center gap-2 mb-4">
      <Magnet class="w-4 h-4 text-purple-400 shrink-0" />
      <input
        v-model="libraryStore.magnet"
        placeholder="Paste a magnet link to download into this library…"
        @keyup.enter="libraryStore.sendMagnet()"
        class="flex-1 bg-gray-700 text-white rounded-md p-2 text-sm"
      />
      <button
        @click="libraryStore.sendMagnet()"
        :disabled="libraryStore.magnetBusy || !libraryStore.magnet.trim()"
        class="btn-primary text-sm"
      >
        <LoadingIcon v-if="libraryStore.magnetBusy" class="w-4 h-4" />
        <Magnet v-else class="w-4 h-4" />
        Send
      </button>
    </div>

    <!-- Season batch-translate progress banner -->
    <div
      v-if="libraryStore.isTranslating && progress"
      class="mb-4 bg-blue-900/40 border border-blue-700 rounded-md p-3 flex items-center gap-3"
    >
      <LoadingIcon class="w-5 h-5 text-blue-400 shrink-0" />
      <div class="flex-1 min-w-0">
        <div class="text-sm text-white font-medium truncate">
          {{ progress.showName }} — {{ progress.seasonName }}
        </div>
        <div class="text-xs text-gray-300 mt-0.5">
          {{ progress.message }}
          <span v-if="progress.totalEpisodes > 0" class="ml-1 text-gray-400">
            ({{ progress.currentEpisode }}/{{ progress.totalEpisodes }})
          </span>
        </div>
        <!-- Progress bar -->
        <div class="h-1.5 bg-gray-700 rounded-full mt-2 overflow-hidden">
          <div
            class="h-full bg-blue-500 rounded-full transition-all duration-300"
            :style="{ width: progress.totalEpisodes > 0 ? (progress.currentEpisode / progress.totalEpisodes * 100) + '%' : '0%' }"
          ></div>
        </div>
      </div>
      <button @click="showStreamModal = true" class="btn-secondary text-xs shrink-0">
        <Eye class="w-3 h-3" />
        View live
      </button>
      <button @click="cancelSeasonTranslate()" class="btn-danger text-xs shrink-0">
        <Square class="w-3 h-3" />
        Cancel
      </button>
    </div>

    <!-- TMDB identification progress banner -->
    <div
      v-if="libraryStore.isIdentifying && identifyProgress"
      class="mb-4 bg-purple-900/40 border border-purple-700 rounded-md p-3 flex items-center gap-3"
    >
      <LoadingIcon class="w-5 h-5 text-purple-400 shrink-0" />
      <div class="flex-1 min-w-0">
        <div class="text-sm text-white font-medium truncate">
          Identifying library via TMDB…
        </div>
        <div class="text-xs text-gray-300 mt-0.5">
          {{ identifyProgress.message }}
          <span v-if="identifyProgress.total > 0" class="ml-1 text-gray-400">
            ({{ identifyProgress.current }}/{{ identifyProgress.total }})
          </span>
        </div>
        <div class="h-1.5 bg-gray-700 rounded-full mt-2 overflow-hidden">
          <div
            class="h-full bg-purple-500 rounded-full transition-all duration-300"
            :style="{ width: identifyProgress.total > 0 ? (identifyProgress.current / identifyProgress.total * 100) + '%' : '0%' }"
          ></div>
        </div>
      </div>
    </div>

    <!-- Empty state -->
    <div v-if="!libraryStore.isScanning && shows.length === 0" class="flex-1 flex flex-col items-center justify-center text-center py-16">
      <Library :size="64" class="text-gray-600 mb-4" />
      <p class="text-gray-400 text-lg mb-2">No library selected</p>
      <p class="text-gray-500 text-sm">Pick a folder to scan for TV shows and movies.</p>
    </div>

    <!-- Loading skeleton -->
    <div v-else-if="libraryStore.isScanning" class="flex-1 flex items-center justify-center">
      <div class="flex flex-col items-center gap-3 text-gray-400">
        <LoadingIcon class="w-8 h-8" />
        <span>Scanning library…</span>
      </div>
    </div>

    <!-- Show tree -->
    <div v-else class="flex-1 overflow-y-auto space-y-2">
      <div
        v-for="show in shows"
        :key="show.path"
        class="bg-gray-800 rounded-md overflow-hidden"
      >
        <!-- Show header -->
        <button
          class="w-full flex items-center gap-3 px-4 py-3 text-left hover:bg-gray-700 transition-colors"
          @click="toggleShow(show)"
        >
          <component :is="isShowExpanded(show) ? ChevronDown : ChevronRight" class="w-4 h-4 text-gray-400 shrink-0" />
          <span class="font-semibold text-white flex-1 truncate">
            {{ show.name }}
            <span v-if="show.year" class="text-gray-400 font-normal ml-1">({{ show.year }})</span>
          </span>
          <span v-if="show.identified" class="text-xs text-purple-400 shrink-0 mr-2" :title="`TMDB #${show.tmdbId}${show.imdbId ? ' · ' + show.imdbId : ''}`">
            TMDB ✓
          </span>
          <span class="text-xs text-gray-400 shrink-0">
            {{ show.seasons.length }} season{{ show.seasons.length !== 1 ? 's' : '' }} · {{ episodeCount(show) }} episodes
          </span>
        </button>

        <!-- Seasons -->
        <div v-if="isShowExpanded(show)" class="border-t border-gray-700">
          <div v-for="season in show.seasons" :key="season.name">
            <!-- Season header -->
            <div class="flex items-center border-b border-gray-700/50">
              <button
                class="flex-1 flex items-center gap-3 px-6 py-2 text-left hover:bg-gray-700/50 transition-colors"
                @click="toggleSeason(show, season)"
              >
                <component :is="isSeasonExpanded(show, season) ? ChevronDown : ChevronRight" class="w-3.5 h-3.5 text-gray-500 shrink-0" />
                <span class="text-sm font-medium text-gray-200">{{ season.name }}</span>
                <span class="text-xs text-gray-500 ml-1">
                  {{ season.episodes.length }} ep
                  <span v-if="translatedCount(season) > 0" class="text-green-500 ml-1">
                    · {{ translatedCount(season) }} translated
                  </span>
                </span>
              </button>
              <!-- Translate whole season button -->
              <button
                v-if="!anyTranslating"
                class="btn-success text-xs py-1 px-2 mr-3 shrink-0"
                :title="'Translate ' + season.name"
                @click.stop="openTranslateModal(show, season)"
              >
                <Languages class="w-3.5 h-3.5" />
                Translate
              </button>
            </div>

            <!-- Episodes -->
            <div v-if="isSeasonExpanded(show, season)">
              <div
                v-for="ep in season.episodes"
                :key="ep.path"
                class="flex items-center gap-3 px-8 py-2 hover:bg-gray-700/30 transition-colors group"
              >
                <CheckCircle
                  v-if="ep.translated"
                  class="w-3.5 h-3.5 text-green-500 shrink-0"
                  :title="'Translated (' + (settingsStore.settings?.defaultTranslationLanguage || '') + ')'"
                />
                <span v-else class="w-3.5 h-3.5 shrink-0" />
                <span class="text-sm text-gray-300 flex-1 truncate">
                  <!-- Show TMDB episode name when available, with the SxxExx tag as a prefix -->
                  <template v-if="ep.identified && ep.episodeName">
                    <span class="text-gray-500 mr-1">{{ ep.name.split('–')[0].trim() }}</span>
                    <span>{{ ep.episodeName }}</span>
                  </template>
                  <template v-else>{{ ep.name }}</template>
                </span>
                <!-- Translating indicator: single-episode run or season run (current episode) -->
                <template
                  v-if="
                    translationStore.activePath === ep.path ||
                    (libraryStore.isTranslating && progress && progress.currentEpisode > 0 && season.episodes[progress.currentEpisode - 1]?.path === ep.path)
                  "
                >
                  <span class="text-xs text-blue-400 shrink-0 flex items-center gap-1">
                    <LoadingIcon class="w-3 h-3" />
                    Translating…
                  </span>
                  <button
                    class="btn-secondary text-xs py-1 px-2 shrink-0"
                    title="Show live translation"
                    @click="showStreamModal = true"
                  >
                    <Eye class="w-3 h-3" />
                    View live
                  </button>
                </template>
                <!-- Per-episode download progress (item is part of an active torrent) -->
                <span
                  v-if="libraryStore.torrentForPath(ep.path)"
                  class="text-xs text-amber-300 shrink-0 flex items-center gap-1"
                  title="Downloading via torrent"
                >
                  ↓ {{ ((libraryStore.torrentForPath(ep.path)!.progress) * 100).toFixed(0) }}%
                </span>
                <!-- Per-episode translate button (hidden while any translation runs) -->
                <button
                  v-else-if="!anyTranslating"
                  class="btn-success text-xs py-1 px-2 opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
                  :title="ep.translated ? 'Re-translate this episode' : 'Translate this episode'"
                  @click="openTranslateModal(show, season, ep)"
                >
                  <Languages class="w-3 h-3" />
                  Translate
                </button>
                <button
                  class="btn-primary text-xs py-1 px-2 opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
                  :disabled="loadingEpisode === ep.path"
                  @click="playEpisode(ep)"
                >
                  <LoadingIcon v-if="loadingEpisode === ep.path" class="w-3 h-3" />
                  <Play v-else class="w-3 h-3" />
                  Play
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Season-translate confirmation modal -->
    <div
      v-if="showTranslateModal && translateTarget"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      @click.self="showTranslateModal = false"
    >
      <div class="bg-gray-800 rounded-md p-6 max-w-md w-full mx-4">
        <h2 class="text-xl font-bold text-white mb-2">
          {{ translateTarget.episode ? "Translate Episode" : "Translate Season" }}
        </h2>
        <p v-if="translateTarget.episode" class="text-gray-400 text-sm mb-4">
          Translate
          <span class="text-white font-medium">{{ translateTarget.episode.episodeName || translateTarget.episode.name }}</span>
          from <span class="text-white font-medium">{{ translateTarget.show.name }}</span>.
        </p>
        <p v-else class="text-gray-400 text-sm mb-4">
          Translate the
          {{ translateTarget.season.episodes.filter((e) => !e.translated).length }}
          untranslated episode(s) of
          <span class="text-white font-medium">{{ translateTarget.season.name }}</span>
          from <span class="text-white font-medium">{{ translateTarget.show.name }}</span>
          sequentially. Already-translated episodes are skipped.
        </p>
        <div class="mb-4">
          <label class="block text-sm text-gray-300 mb-1">Target Language</label>
          <input
            v-model="translateLanguage"
            type="text"
            placeholder="e.g. Turkish"
            class="w-full bg-gray-700 text-white rounded-md p-2 text-sm"
          />
        </div>
        <div class="flex gap-2 justify-end">
          <button @click="showTranslateModal = false" class="btn-secondary text-sm">Cancel</button>
          <button
            @click="startTranslate()"
            :disabled="!translateLanguage.trim()"
            class="btn-primary text-sm"
          >
            <Languages class="w-4 h-4" />
            Start Translation
          </button>
        </div>
      </div>
    </div>

    <!-- Organize preview modal -->
    <div
      v-if="showOrganizeModal && organizePlan.length > 0"
      class="fixed inset-0 bg-black/60 flex items-center justify-center z-50"
      @click.self="cancelOrganize()"
    >
      <div class="bg-gray-800 rounded-md p-6 max-w-2xl w-full mx-4 flex flex-col max-h-[80vh]">
        <!-- Modal header -->
        <div class="flex items-center justify-between mb-3">
          <h2 class="text-xl font-bold text-white flex items-center gap-2">
            <FolderCheck class="w-6 h-6 text-purple-400" />
            Organize Preview
          </h2>
          <button @click="cancelOrganize()" class="text-gray-400 hover:text-white transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>
        <p class="text-gray-400 text-sm mb-4">
          {{ organizePlan.length }} file{{ organizePlan.length !== 1 ? 's' : '' }} will be moved into the canonical layout.
          Files are <strong class="text-white">moved in-place</strong> — nothing is deleted.
          Sibling subtitle directories are moved with their video.
        </p>

        <!-- Scrollable move list -->
        <div class="flex-1 overflow-y-auto space-y-1 mb-4 min-h-0">
          <div
            v-for="(move, idx) in organizePlan"
            :key="idx"
            class="bg-gray-700/50 rounded px-3 py-2 text-xs"
          >
            <div class="text-gray-300 truncate">{{ move.description }}</div>
            <div class="text-gray-500 truncate mt-0.5">
              <span class="text-gray-400">from:</span> {{ move.srcVideo }}
            </div>
            <div v-if="move.srcSubDir" class="text-purple-400/70 truncate mt-0.5">
              + subtitle dir: {{ move.srcSubDir.split('/').pop() }}
            </div>
          </div>
        </div>

        <!-- Actions -->
        <div class="flex gap-2 justify-end">
          <button @click="cancelOrganize()" class="btn-secondary text-sm">Cancel</button>
          <button
            @click="confirmOrganize()"
            :disabled="libraryStore.isOrganizing"
            class="btn-primary text-sm"
          >
            <LoadingIcon v-if="libraryStore.isOrganizing" class="w-4 h-4" />
            <FolderCheck v-else class="w-4 h-4" />
            Move {{ organizePlan.length }} File{{ organizePlan.length !== 1 ? 's' : '' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Live translation stream modal -->
    <div
      v-if="showStreamModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      @click.self="showStreamModal = false"
    >
      <div class="bg-gray-800 rounded-md p-6 max-w-3xl w-full mx-4 max-h-[80vh] flex flex-col">
        <div class="flex items-center justify-between mb-3">
          <h2 class="text-xl font-bold text-white flex items-center gap-2">
            <Languages class="w-5 h-5 text-blue-400" />
            Live Translation
          </h2>
          <button @click="showStreamModal = false" class="text-gray-400 hover:text-white transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>

        <!-- Status row -->
        <div class="flex items-center gap-3 mb-3">
          <template v-if="anyTranslating">
            <div class="bg-gray-700 text-white rounded-md px-4 py-2 flex items-center text-sm">
              <LoadingIcon class="w-4 h-4 mr-2" />
              <span v-if="libraryStore.isTranslating && progress">
                Translating {{ progress.seasonName }}
                <span v-if="progress.totalEpisodes > 0" class="text-gray-400 ml-1">
                  ({{ progress.currentEpisode }}/{{ progress.totalEpisodes }})
                </span>
              </span>
              <span v-else>Translating…</span>
            </div>
            <div class="flex-1"></div>
            <button @click="cancelActiveTranslation()" class="btn-danger text-sm">
              <Square class="w-4 h-4" />
              Cancel
            </button>
          </template>
          <template v-else>
            <div class="text-sm text-gray-400">Translation finished.</div>
          </template>
        </div>

        <!-- Stream output -->
        <div class="flex-1 overflow-y-auto bg-gray-900 rounded-md p-4 font-mono text-sm text-green-400 min-h-[8rem]">
          <pre v-if="translationStore.streamContent" class="whitespace-pre-wrap text-left">{{ translationStore.streamContent }}</pre>
          <span v-else class="text-gray-500">Waiting for model output…</span>
        </div>
      </div>
    </div>

  </div>
</template>
