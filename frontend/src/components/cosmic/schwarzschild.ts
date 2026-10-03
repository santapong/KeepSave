/**
 * Null geodesics for a distant, stationary observer; G = c = M = 1.
 * u = 1/r, u'' = 3u² - u, u(0) = 0, u'(0) = 1 / impact.
 * Horizon r = 2, photon sphere r = 3, critical impact = sqrt(27).
 * See docs/design/BLACK_HOLE_LANDING.md for references and limits.
 */
export const RAY_TABLE_WIDTH = 1024;
export const RAY_TABLE_HEIGHT = 512;
export const MAX_IMPACT = 32;
export const MAX_PHI = 3 * Math.PI;
export const CRITICAL_IMPACT = Math.sqrt(27);

/** Values after capture/escape remain 0.5/0 respectively (no disk emission). */
export function photonOrbit(impact: number, samples: number, maxPhi: number): Float32Array {
  const orbit = new Float32Array(samples);
  const step = maxPhi / (samples - 1) / 2;
  let u = 0;
  let velocity = 1 / impact;
  let stopped = false;
  const acceleration = (radius: number) => 3 * radius * radius - radius;
  for (let i = 1; i < samples; i++) {
    for (let sub = 0; sub < 2 && !stopped; sub++) {
      // RK4, with two substeps per stored angular sample.
      const a1 = acceleration(u);
      const v2 = velocity + step * a1 / 2;
      const a2 = acceleration(u + step * velocity / 2);
      const v3 = velocity + step * a2 / 2;
      const a3 = acceleration(u + step * v2 / 2);
      const v4 = velocity + step * a3;
      const a4 = acceleration(u + step * v3);
      u += step * (velocity + 2 * v2 + 2 * v3 + v4) / 6;
      velocity += step * (a1 + 2 * a2 + 2 * a3 + a4) / 6;
      if (u >= .5) { u = .5; stopped = true; }
      else if (u < 0) { u = 0; stopped = true; }
    }
    orbit[i] = u;
  }
  return orbit;
}

let cachedTable: Float32Array | undefined;

/** One reusable 2 MiB CPU table; the GPU copy belongs to each scene's lifetime. */
export function getRayTable(): Float32Array {
  if (cachedTable) return cachedTable;
  const table = new Float32Array(RAY_TABLE_WIDTH * RAY_TABLE_HEIGHT);
  for (let x = 0; x < RAY_TABLE_WIDTH; x++) {
    // Concentrate samples at the critical curve, where light paths vary rapidly.
    const coordinate = x / (RAY_TABLE_WIDTH - 1);
    const impact = Math.max(.05, coordinate < .4
      ? CRITICAL_IMPACT * (1 - (1 - coordinate / .4) ** 2)
      : CRITICAL_IMPACT + (MAX_IMPACT - CRITICAL_IMPACT) * ((coordinate - .4) / .6) ** 2);
    const orbit = photonOrbit(impact, RAY_TABLE_HEIGHT, MAX_PHI);
    for (let y = 0; y < RAY_TABLE_HEIGHT; y++) table[y * RAY_TABLE_WIDTH + x] = orbit[y];
  }
  cachedTable = table;
  return table;
}
