import { ref } from "vue";

// A "source" is where library media + playback live: this machine (local) or a
// discovered remote wails-cast instance reached over its HTTP API. The app shows
// every source's library and lets you play from any of them; the SAME local UI
// drives both — only the backend target changes (VSCode-Remote style).

export interface Source {
  kind: "local" | "remote";
  name: string; // "This Mac" or the remote instance name
  base: string; // "" for local, e.g. "http://fedora.local:9999" for remote
  token: string; // remote auth token ("" = none / local)
}

export const LOCAL_SOURCE: Source = {
  kind: "local",
  name: "This Mac",
  base: "",
  token: "",
};

// activeSource is the source that PLAYBACK currently targets (set when you press
// Play). Transport/volume/subtitle controls route to this source's backend.
// Browsing a library uses the library store's own browse source, which may
// differ from what is currently playing.
export const activeSource = ref<Source>(LOCAL_SOURCE);

export function isRemoteActive(): boolean {
  return activeSource.value.kind === "remote";
}

export function sourceKey(s: Source): string {
  return s.kind === "local" ? "local" : `${s.base}`;
}
