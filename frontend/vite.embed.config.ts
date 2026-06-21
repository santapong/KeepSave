import { defineConfig } from 'vite';
import { resolve } from 'path';

export default defineConfig({
  build: {
    lib: {
      entry: resolve(__dirname, 'src/embed/index.ts'),
      name: 'KeepSave',
      formats: ['es', 'umd'],
      fileName: (format) => `keepsave-widget.${format}.js`,
    },
    outDir: 'dist-embed',
    emptyOutDir: true,
    // FE-F05: the embed widget is shipped to third-party integrators. Do NOT
    // emit source maps for it — they would expose internal SDK source.
    sourcemap: false,
  },
});
