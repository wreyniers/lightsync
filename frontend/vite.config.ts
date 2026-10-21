import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "path";
import { getAppVersion } from "../scripts/app-version.mjs";

const appVersion = getAppVersion();

export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(appVersion),
  },
  plugins: [react(), tailwindcss()],
  server: {
    host: "127.0.0.1",
  },
  resolve: {
    alias: {
      "@bindings/internal": path.resolve(__dirname, "./bindings/lightsync/internal"),
      "@bindings": path.resolve(__dirname, "./bindings/lightsync/index.js"),
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
