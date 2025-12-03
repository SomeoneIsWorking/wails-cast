import { createApp } from "vue";
import { createPinia } from "pinia";
import App from "./App.vue";
import "./style.css";
import Toast, { POSITION, useToast } from "vue-toastification";
import "vue-toastification/dist/index.css";
import type { PluginOptions } from "vue-toastification";
import { useSettingsStore } from "./stores/settings";
import { setupLoggingHandlers } from "./setupLoggingHandlers";


export const app = createApp(App);
export const toast = useToast();

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
setupLoggingHandlers();

app.use(createPinia());

await useSettingsStore().loadSettings();
setupLoggingHandlers();

app.mount("#app");
