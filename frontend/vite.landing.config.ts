import { defineConfig, type Plugin } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { readFileSync } from 'node:fs';
import path from 'node:path';

const frontend = import.meta.dirname;
const assets = ['keepsave.svg', 'keepsave-light.svg', 'keepsave-favicon.svg', 'images/gargantua-horizon.webp'];

function publicLanding(): Plugin {
  return {
    name: 'public-landing-boundary',
    generateBundle(_options, bundle) {
      for (const output of Object.values(bundle)) {
        if (output.type !== 'chunk') continue;
        for (const id of Object.keys(output.modules)) {
          const relative = path.relative(path.join(frontend, 'src'), id).replaceAll('\\', '/');
          if (relative.startsWith('api/') || relative.startsWith('hooks/') || relative === 'App.tsx'
            || (relative.startsWith('pages/') && relative !== 'pages/LandingPage.tsx')) {
            this.error(`Private application module in landing build: ${relative}`);
          }
        }
      }
      for (const fileName of assets) {
        this.emitFile({ type: 'asset', fileName, source: readFileSync(path.join(frontend, 'public', fileName)) });
      }
    },
  };
}

export default defineConfig({
  root: path.join(frontend, 'landing'),
  publicDir: false,
  envDir: false,
  plugins: [react(), tailwindcss(), publicLanding()],
  resolve: { alias: { '@': path.join(frontend, 'src') } },
  build: { outDir: path.join(frontend, 'dist-landing'), emptyOutDir: true },
});
