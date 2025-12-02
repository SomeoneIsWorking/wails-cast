import { createApp } from "vue";
import { createPinia } from "pinia";
import App from "./App.vue";
import "./style.css";
import { logger } from "./utils/logger";
import { OnFileDrop } from "../wailsjs/runtime/runtime";
import Toast, { POSITION, useToast } from "vue-toastification";
import "vue-toastification/dist/index.css";
import type { PluginOptions } from "vue-toastification";

const app = createApp(App);
const toast = useToast();

// Toast configuration
const toastOptions: PluginOptions = {
  position: POSITION.TOP_LEFT,
  timeout: 3000,
  closeOnClick: true,
  pauseOnFocusLoss: true,
  pauseOnHover: true,
  draggable: true,
  draggablePercent: 0.6,
  showCloseButtonOnHover: false,
  hideProgressBar: false,
  closeButton: "button",
  icon: true,
  rtl: false,
};

app.use(Toast, toastOptions);

// Redirect console methods to Go backend
const originalLog = console.log;
const originalWarn = console.warn;
const originalError = console.error;

console.log = (...args: unknown[]) => {
  const message = args
    .map((arg) => (typeof arg === "string" ? arg : JSON.stringify(arg)))
    .join(" ");
  logger.info(message);
  originalLog(...args);
};

console.warn = (...args: unknown[]) => {
  const message = args
    .map((arg) => (typeof arg === "string" ? arg : JSON.stringify(arg)))
    .join(" ");
  logger.warn(message);
  originalWarn(...args);
};

console.error = (...args: unknown[]) => {
  const message = args
    .map((arg) => (typeof arg === "string" ? arg : JSON.stringify(arg)))
    .join(" ");
  logger.error(message);
  originalError(...args);
};

// Global Vue error handler
app.config.errorHandler = (err, _instance, info) => {
  const errorMessage = err instanceof Error ? err.message : String(err);
  logger.error(`[App Error] ${info}`, errorMessage);
  originalError(`[App Error] ${info}:`, err);
  toast.error(errorMessage);
};

// Global warning handler
app.config.warnHandler = (msg, _instance, trace) => {
  logger.warn(`[App Warning]`, msg);
  originalWarn(`[App Warning]:`, msg, trace);
};

// Unhandled promise rejection
window.addEventListener("unhandledrejection", (event) => {
  const errorMessage = event.reason instanceof Error ? event.reason.message : String(event.reason);
  logger.error("[Unhandled Promise Rejection]", errorMessage);
  originalError("[Unhandled Promise Rejection]:", event.reason);
  toast.error(errorMessage);
  event.preventDefault();
});

app.use(createPinia());
app.mount("#app");

const centralDropHandler = (x: number, y: number, paths: string[]) => {
  window.dispatchEvent(
    new CustomEvent("wails-file-drop", { detail: { x, y, paths } })
  );
};
OnFileDrop(centralDropHandler, true);
