import { qualityOptions } from "./qualityOptions";
import { SettingCategory } from "../stores/settings";

// Setting categories for organization
export const settingCategories: SettingCategory[] = [
  {
    id: "subtitles",
    label: "Subtitles",
    icon: "Subtitles",
    settings: [
      {
        key: "subtitleBurnIn",
        label: "Burn-in Subtitles",
        description: "Burn subtitles into video stream",
        type: "boolean",
      },
      {
        key: "ignoreClosedCaptions",
        label: "Ignore Closed Captions",
        description: "Remove closed captions from subtitles",
        type: "boolean",
      },
      {
        key: "subtitleFontSize",
        label: "Font Size",
        description: "Default subtitle font size",
        type: "number",
        min: 10,
        max: 400,
        step: 10,
      },
      {
        key: "subtitleBold",
        label: "Bold Subtitles",
        description: "Render subtitles in bold",
        type: "boolean",
      },
      {
        key: "subtitleItalic",
        label: "Italic Subtitles",
        description: "Render subtitles in italic",
        type: "boolean",
      },
    ],
  },
  {
    id: "translation",
    label: "Translation",
    icon: "Languages",
    settings: [
      {
        key: "defaultTranslationLanguage",
        label: "Default Target Language",
        description: "Default language for subtitle translation",
        type: "text",
      },
    ],
  },
  {
    id: "ai",
    label: "LLM",
    icon: "Brain",
    settings: [
      {
        key: "llmProvider",
        label: "LLM Provider",
        description: "Which AI backend to use for translation",
        type: "select",
        options: [
          { value: "opencode", label: "OpenCode (default)" },
          { value: "openai-compat", label: "OpenAI-compatible endpoint" },
        ],
      },
      {
        key: "llmApiKey",
        label: "API Key",
        description: "API key for the selected provider. For OpenCode, defaults to opencode's auth.json if empty. For OpenAI-compatible, this is the Bearer token.",
        type: "password",
      },
      {
        key: "llmModel",
        label: "Model",
        description: "Model to use for translation. Loaded dynamically from the selected provider.",
        type: "dynamic-select",
        dynamicOptionsProvider: "llmProvider",
      },
      {
        key: "llmBaseURL",
        label: "Base URL",
        description: "Base URL for the OpenAI-compatible endpoint (e.g. http://localhost:11434). Only used when provider is OpenAI-compatible endpoint.",
        type: "text",
      },
      // --- shared ---
      {
        key: "tmdbApiKey",
        label: "TMDB API Key",
        description: "Your TMDB v3 API key for show/episode identification. Get one free at themoviedb.org.",
        type: "password",
      },
      {
        key: "translatePromptTemplate",
        label: "Translation Prompt Template",
        description: "Custom prompt template for subtitle translation. Use {{.TargetLanguage}} and {{.SubtitleContent}} as placeholders.",
        type: "textarea",
      },
      {
        key: "maxSubtitleSamples",
        label: "Max Subtitle Samples",
        description: "Maximum number of reference subtitle tracks to use for translation",
        type: "number",
        min: 1,
        max: 10,
        step: 1,
      },
    ],
  },
  {
    id: "quality",
    label: "Quality",
    icon: "Settings",
    settings: [
      {
        key: "defaultQuality",
        label: "Default Quality",
        description: "Default quality preset for video encoding",
        type: "select",
        options: qualityOptions,
      },
      {
        key: "maxOutputWidth",
        label: "Max Output Width",
        description: "Maximum output width for video encoding (0 for original width)",
        type: "number",
        min: 640,
        max: 3840,
        step: 1,
      },
    ],
  },
  {
    id: "cache",
    label: "Cache",
    icon: "HardDrive",
    settings: [
      {
        key: "noTranscodeCache",
        label: "Disable Transcoding Cache",
        description: "Disable caching of transcoded video segments",
        type: "boolean",
      },
    ],
  },
  {
    id: "remote",
    label: "Remote API",
    icon: "Smartphone",
    settings: [
      {
        key: "remoteApiEnabled",
        label: "Enable Remote API",
        description: "Run an HTTP server so companion apps (e.g. Android) can browse the library and trigger playback. Restart required to apply.",
        type: "boolean",
      },
      {
        key: "remoteApiPort",
        label: "Remote API Port",
        description: "Port the HTTP server listens on",
        type: "number",
        min: 1024,
        max: 65535,
        step: 1,
      },
      {
        key: "remoteApiToken",
        label: "Remote API Token",
        description: "Optional shared secret. When set, clients must send it as the X-Cast-Token header. Leave blank for open LAN access.",
        type: "password",
      },
    ],
  },
];
