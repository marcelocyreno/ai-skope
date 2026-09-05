// Build pass 2: the content script.
//
// MV3 content scripts cannot be ES modules, so this emits a single IIFE with
// everything inlined. Its CSS is imported as a string and injected into a
// Shadow DOM at runtime, so the page's styles and ours cannot reach each other.
import { defineConfig } from "vite";
import { fileURLToPath, URL } from "node:url";

export default defineConfig({
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  build: {
    outDir: "dist",
    emptyOutDir: false,
    target: "chrome116",
    lib: {
      entry: fileURLToPath(new URL("./src/content/index.ts", import.meta.url)),
      formats: ["iife"],
      name: "AISkopeContent",
      fileName: () => "content.js",
    },
    rollupOptions: { output: { inlineDynamicImports: true } },
  },
});
