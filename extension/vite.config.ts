// Build pass 1: the two extension pages (side panel, options).
//
// These are ordinary documents, so ES modules are fine and Vite's HTML
// handling applies. The content script and the service worker need different
// output shapes and are built by their own configs.
import { defineConfig, type Plugin } from "vite";
import vue from "@vitejs/plugin-vue";
import { fileURLToPath, URL } from "node:url";
import { copyFileSync, mkdirSync, existsSync, readdirSync } from "node:fs";

/**
 * Chrome loads the unpacked extension from dist/, so the manifest and any
 * static icons have to land there. Keeping manifest.json at the project root
 * (where a reader expects it) means copying it in at the end of the build.
 */
function copyExtensionFiles(): Plugin {
  return {
    name: "copy-extension-files",
    closeBundle() {
      mkdirSync("dist", { recursive: true });
      copyFileSync("manifest.json", "dist/manifest.json");
      if (existsSync("icons")) {
        mkdirSync("dist/icons", { recursive: true });
        for (const f of readdirSync("icons")) copyFileSync(`icons/${f}`, `dist/icons/${f}`);
      }
    },
  };
}

export default defineConfig({
  plugins: [vue(), copyExtensionFiles()],
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    target: "chrome116", // the Side Panel API landed in 114; 116 is a safe floor
    rollupOptions: {
      input: {
        sidepanel: fileURLToPath(new URL("./sidepanel.html", import.meta.url)),
        options: fileURLToPath(new URL("./options.html", import.meta.url)),
      },
      output: {
        entryFileNames: "[name].js",
        chunkFileNames: "chunks/[name]-[hash].js",
        assetFileNames: "assets/[name][extname]",
      },
    },
  },
});
