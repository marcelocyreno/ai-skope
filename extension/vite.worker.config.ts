// Build pass 3: the service worker.
//
// Declared with "type": "module" in the manifest, so an ES module is correct
// here — but it must be a single file, since a worker cannot import from a
// chunk directory that Chrome has not been told about.
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
      entry: fileURLToPath(new URL("./src/worker/service-worker.ts", import.meta.url)),
      formats: ["es"],
      fileName: () => "service-worker.js",
    },
    rollupOptions: { output: { inlineDynamicImports: true } },
  },
});
