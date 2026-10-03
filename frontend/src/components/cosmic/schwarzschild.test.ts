import { describe, expect, it } from 'vitest';
import { CRITICAL_IMPACT, getRayTable, photonOrbit, RAY_TABLE_HEIGHT, RAY_TABLE_WIDTH } from './schwarzschild';

describe('Schwarzschild photon paths', () => {
  it('captures rays below the analytic critical impact and lets rays above it escape', () => {
    const captured = photonOrbit(CRITICAL_IMPACT * .99, 2048, 3 * Math.PI);
    const escaped = photonOrbit(CRITICAL_IMPACT * 1.01, 2048, 3 * Math.PI);
    expect(captured.at(-1)).toBe(.5); // r = 2M, event horizon
    expect(escaped.at(-1)).toBe(0); // escaped to infinity
    expect(Math.max(...escaped)).toBeLessThan(1 / 3); // turns outside photon sphere
  });
  it('approaches the photon sphere for the critical ray', () => {
    const orbit = photonOrbit(CRITICAL_IMPACT, 2048, 8);
    expect(orbit.at(-1)).toBeCloseTo(1 / 3, 3);
  });
  it('recovers the weak-field 4M/b deflection limit', () => {
    const samples = 8192;
    const maxPhi = 4;
    const orbit = photonOrbit(100, samples, maxPhi);
    const escapeIndex = orbit.findIndex((u, i) => i > 0 && u === 0);
    const deflection = escapeIndex * maxPhi / (samples - 1) - Math.PI;
    expect(deflection).toBeGreaterThan(.04);
    expect(deflection).toBeLessThan(.042);
  });
  it('converges under angular step refinement away from capture/escape boundaries', () => {
    const coarse = photonOrbit(7, 512, 3 * Math.PI);
    const fine = photonOrbit(7, 1023, 3 * Math.PI);
    for (let i = 0; i < 180; i++) expect(Math.abs(coarse[i] - fine[i * 2])).toBeLessThan(1e-6);
  });
  it('caches one finite, bounded lookup instead of rebuilding on animation frames', () => {
    const table = getRayTable();
    expect(table).toBe(getRayTable());
    expect(table.length).toBe(RAY_TABLE_WIDTH * RAY_TABLE_HEIGHT);
    expect(table.byteLength).toBe(2 * 1024 * 1024);
    expect(table.every(u => Number.isFinite(u) && u >= 0 && u <= .5)).toBe(true);
  });
});
