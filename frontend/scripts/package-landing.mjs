import { cp, mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { createHash } from 'node:crypto';

const frontend = path.resolve(import.meta.dirname, '..');
const build = path.join(frontend, 'dist-landing');
const deployment = path.resolve(frontend, '../deploy/landing');
const output = path.join(deployment, '.vercel/output');
const allowed = /^(index\.html|404\.html|robots\.txt|sitemap\.xml|keepsave(?:-light|-favicon)?\.svg|images\/gargantua-horizon\.webp|assets\/[\w-]+\.(js|css))$/;

async function files(directory, prefix = '') {
  const entries = await readdir(directory, { withFileTypes: true });
  const result = [];
  for (const entry of entries) {
    const name = prefix + entry.name;
    if (entry.isDirectory()) result.push(...await files(path.join(directory, entry.name), `${name}/`));
    else if (entry.isFile()) result.push(name);
    else throw new Error(`Unexpected landing artifact: ${name}`);
  }
  return result.sort();
}

await cp(path.join(frontend, 'landing/404.html'), path.join(build, '404.html'));
await writeFile(path.join(build, 'robots.txt'), 'User-agent: *\nAllow: /\nSitemap: https://keepsave.draveniq.dev/sitemap.xml\n');
await writeFile(path.join(build, 'sitemap.xml'), '<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>https://keepsave.draveniq.dev/</loc></url></urlset>\n');
const manifest = {};
for (const name of await files(build)) {
  if (!allowed.test(name)) throw new Error(`Non-public artifact refused: ${name}`);
  manifest[name] = createHash('sha256').update(await readFile(path.join(build, name))).digest('hex');
}
// Keep CLI project metadata intact while replacing only this generated output.
await rm(output, { recursive: true, force: true });
await mkdir(output, { recursive: true });
await cp(build, path.join(output, 'static'), { recursive: true });
await cp(path.join(deployment, 'output-config.json'), path.join(output, 'config.json'));
await writeFile(path.join(output, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n');
console.log(`Packaged ${Object.keys(manifest).length} public files for the landing deployment.`);
