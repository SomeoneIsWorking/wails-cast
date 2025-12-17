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
        description: "Default font size for burned-in subtitles",
        type: "number",
        min: 12,
        max: 72,
        step: 2,
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
    label: "AI Configuration",
    icon: "Brain",
    settings: [
      {
        key: "geminiApiKey",
        label: "Gemini API Key",
        description: "Your Google Gemini API key for AI features",
        type: "password",
      },
      {
        key: "geminiModel",
        label: "Gemini Model",
        description: "Which Gemini model to use",
        type: "text",
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
];
