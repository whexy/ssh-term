import { defineConfig } from "vite";

export default defineConfig({
  build: {
    target: "esnext",
    rollupOptions: {
      output: {
        entryFileNames: "main.js",
        assetFileNames: "assets/[name][extname]",
      },
    },
  },
});
