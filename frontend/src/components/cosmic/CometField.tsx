import { useEffect, useRef, useState } from 'react';
import type * as THREE_NS from 'three';
import { prefersReducedMotion as motionOff } from '@/lib/motion';

/**
 * CometField — a few real comets on real orbits, with real dust tails.
 *
 * The WebGL backdrop for the login and register screens, and the companion
 * to the landing page's <Singularity>. That one ray-traces null geodesics
 * to show the hole; this one integrates matter to show what falls in.
 *
 * Deliberately only a handful of comets. Each one is expensive because it
 * is modelled properly rather than drawn, and a sky with four detailed
 * comets reads far better than one with thirty tokens.
 *
 * ── 1. Orbits from orbital elements ───────────────────────────────────
 * Each nucleus is defined by the six Keplerian elements an astronomer
 * would actually quote — semi-major axis a, eccentricity e, inclination i,
 * longitude of ascending node Ω, argument of perihelion ω, and mean
 * anomaly at epoch M₀ — not by a hand-picked velocity vector.
 *
 * Converting elements to a state vector requires solving Kepler's
 * equation, M = E − e·sin E, which has no closed form. It is solved here
 * by Newton–Raphson, which converges in a handful of iterations even at
 * the high eccentricities real comets have (e ≈ 0.6–0.97 below; Halley is
 * 0.967).
 *
 * The nuclei then integrate under
 *
 *     a = −(GM/r³)·r·(1 + 3h²/(c²r²))
 *            └ Newton ┘  └ relativistic correction ┘
 *
 * so the ellipses precess — the Mercury-perihelion effect. The c²
 * denominator is not decoration: drop it and the "correction" evaluates to
 * ~12× the Newtonian term at these radii and the orbits degenerate.
 * Integration is velocity Verlet, which is symplectic, so orbital energy
 * stays bounded instead of drifting the way forward Euler does.
 *
 * ── 2. Tails from Finson–Probstein dust dynamics ──────────────────────
 * This is the part that makes the tails real rather than drawn.
 *
 * A dust grain leaving the nucleus feels radiation pressure pushing it
 * outward while gravity pulls it in. The ratio of the two is β, and it
 * depends only on grain size — so a grain moves under *reduced* gravity:
 *
 *     a_grain = −GM(1 − β)·r̂ / r²
 *
 * Every grain therefore follows its own Keplerian orbit around the same
 * mass, just with a weaker μ. Integrating a population of grains with a
 * spread of β, released continuously along the nucleus's path, *is* the
 * Finson–Probstein model — the standard treatment of cometary dust. The
 * curved dust tail and the straight ion tail are not two hand-drawn
 * shapes here; they are one mechanism at two ends of the β range:
 *
 *   · dust  β ≈ 0.05–0.9  — barely unbound, keeps most of the orbital
 *     velocity it was released with, so it lags into the classic curve.
 *   · ions  β ≈ 6–16      — swept almost straight anti-radially, because
 *     the outward force overwhelms both gravity and the release velocity.
 *
 * Emission rate scales as 1/r², so a comet is nearly bare far out and
 * blooms through perihelion. That is also why it is worth watching.
 *
 * Degradations:
 *   · reduced motion  -> one settled static frame, then nothing moves.
 *   · no WebGL / three fails -> renders nothing; the CSS <Starfield>
 *     behind it still carries the screen.
 *   · scrolled out of view -> the rAF loop parks itself.
 */

interface CometFieldProps {
  className?: string;
}

/* --- population ----------------------------------------------------
   GRAINS must be >= peak emission rate x longest lifetime, or the ring
   buffer overwrites grains that are still alive and the tail is truncated
   exactly at perihelion, when it should be longest.
   Peak: 350/s x 6.0s = 2100.

   Grain-nucleus separation grows as ~(1/2)(beta.GM/r^2)t^2, so tail LENGTH
   is dominated by how long grains live, not by how many there are. Short
   lifetimes give a stubby coma no matter how hard they are pushed. */
const GRAINS = 2200;
const EMIT_PEAK_PER_SEC = 350;
const DUST_LIFE_MAX = 6.0;

/* --- units ---------------------------------------------------------
   Camera at CAM_Z with a 50° vertical fov: visible half-height at the
   origin is ~0.466·CAM_Z. Orbits are sized to sweep inside that. */
const CAM_Z = 34;
const GM = 90;
/** c² in these units. The PN term is a correction of order (v/c)², so c
 *  must sit well above orbital speed (~3) or it stops being a correction
 *  and starts being the whole force. */
const C2 = 900;

const VERT = /* glsl */ `
  attribute float aSize;
  attribute float aAlpha;
  attribute vec3  aColor;
  uniform float uPixelRatio;
  varying float vAlpha;
  varying vec3  vColor;
  void main() {
    vAlpha = aAlpha;
    vColor = aColor;
    vec4 mv = modelViewMatrix * vec4(position, 1.0);
    gl_PointSize = aSize * uPixelRatio * (62.0 / -mv.z);
    gl_Position = projectionMatrix * mv;
  }
`;

const FRAG = /* glsl */ `
  precision mediump float;
  varying float vAlpha;
  varying vec3  vColor;
  void main() {
    float d = length(gl_PointCoord - vec2(0.5));
    if (d > 0.5) discard;
    float falloff = pow(1.0 - d * 2.0, 2.4);
    gl_FragColor = vec4(vColor, falloff * vAlpha);
  }
`;

/** Six Keplerian elements. Angles in radians. */
interface Elements {
  a: number;
  e: number;
  i: number;
  raan: number; // Ω, longitude of ascending node
  argp: number; // ω, argument of perihelion
  m0: number;   // mean anomaly at epoch
}

/* Four comets, each on a distinctly eccentric orbit so they visibly swing
   through perihelion and bloom. Perihelion q = a(1−e), aphelion Q = a(1+e). */
/* Perihelion q = a(1-e) must stay OUTSIDE the black hole's apparent
   shadow, or comets visibly dive through it. The shadow subtends roughly
   2.6 geometric units in <Singularity>'s frame, which works out near 3
   units in this field's scale — so every q below is kept comfortably
   above that while staying eccentric enough to bloom at perihelion. */
const ORBITS: Elements[] = [
  { a: 10.5, e: 0.62, i: 0.22, raan: 0.4, argp: 1.1, m0: 0.0 },  // q 4.0  Q 17.0
  { a: 9.0, e: 0.66, i: -0.35, raan: 2.1, argp: 2.6, m0: 2.3 },  // q 3.1  Q 14.9
  { a: 12.5, e: 0.55, i: 0.14, raan: 4.0, argp: 0.3, m0: 4.1 },  // q 5.6  Q 19.4
  { a: 8.0, e: 0.62, i: 0.45, raan: 5.2, argp: 3.9, m0: 1.2 },   // q 3.0  Q 13.0
];
const COMETS = ORBITS.length;
const POINTS = COMETS * (1 + GRAINS);

/**
 * Solve Kepler's equation M = E − e·sin E for the eccentric anomaly E.
 * No closed form exists, so: Newton–Raphson on f(E) = E − e·sin E − M.
 * Seeded with E = M + e·sin M, which keeps it converging quickly even at
 * the high eccentricities comets actually have.
 */
function solveKepler(M: number, e: number): number {
  let E = M + e * Math.sin(M);
  for (let n = 0; n < 12; n++) {
    const f = E - e * Math.sin(E) - M;
    const fp = 1 - e * Math.cos(E);
    const dE = f / fp;
    E -= dE;
    if (Math.abs(dE) < 1e-10) break;
  }
  return E;
}

export function CometField({ className }: CometFieldProps) {
  const hostRef = useRef<HTMLDivElement>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;

    let disposed = false;
    let raf = 0;
    let cleanup: (() => void) | undefined;

    async function boot() {
      let THREE: typeof THREE_NS;
      try {
        THREE = await import('three');
      } catch {
        if (!disposed) setFailed(true);
        return;
      }
      if (disposed || !host) return;

      let renderer: THREE_NS.WebGLRenderer;
      try {
        renderer = new THREE.WebGLRenderer({
          alpha: true,
          antialias: false,
          // Additive blending accumulates colour but not alpha; without
          // premultipliedAlpha:false plus the alpha-accumulating blend
          // below, the whole field composites at alpha 0 and vanishes.
          premultipliedAlpha: false,
          powerPreference: 'low-power',
        });
      } catch {
        setFailed(true);
        return;
      }

      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      renderer.setPixelRatio(dpr);
      renderer.setClearColor(0x000000, 0);

      const measure = () => {
        const r = host.getBoundingClientRect();
        return { w: Math.max(1, Math.round(r.width)), h: Math.max(1, Math.round(r.height)) };
      };
      const first = measure();
      renderer.setSize(first.w, first.h, false);

      const el = renderer.domElement;
      el.style.width = '100%';
      el.style.height = '100%';
      el.style.display = 'block';
      host.appendChild(el);

      const scene = new THREE.Scene();
      const camera = new THREE.PerspectiveCamera(50, first.w / first.h, 0.1, 200);
      camera.position.set(0, 0, CAM_Z);
      camera.lookAt(0, 0, 0);

      /* --- nuclei ------------------------------------------------- */
      const nx = new Float32Array(COMETS);
      const ny = new Float32Array(COMETS);
      const nz = new Float32Array(COMETS);
      const nvx = new Float32Array(COMETS);
      const nvy = new Float32Array(COMETS);
      const nvz = new Float32Array(COMETS);
      const nax = new Float32Array(COMETS);
      const nay = new Float32Array(COMETS);
      const naz = new Float32Array(COMETS);

      /* --- grains (dust + ions share one array) ------------------- */
      const G = COMETS * GRAINS;
      const gx = new Float32Array(G);
      const gy = new Float32Array(G);
      const gz = new Float32Array(G);
      const gvx = new Float32Array(G);
      const gvy = new Float32Array(G);
      const gvz = new Float32Array(G);
      const gbeta = new Float32Array(G);
      const gage = new Float32Array(G);
      const glife = new Float32Array(G);
      const cursor = new Int32Array(COMETS); // ring-buffer write head per comet

      const positions = new Float32Array(POINTS * 3);
      const sizes = new Float32Array(POINTS);
      const alphas = new Float32Array(POINTS);
      const colors = new Float32Array(POINTS * 3);

      const C_CORE = new THREE.Color('#ffffff');
      const C_DUST_BIG = new THREE.Color('#ffc07a');  // low beta, big grains
      const C_DUST_FINE = new THREE.Color('#ffe9c9'); // higher beta
      const C_ION = new THREE.Color('#7fc7ff');
      const tmp = new THREE.Color();

      /** Elements -> state vector, via Kepler's equation. */
      const stateFromElements = (o: Elements, i: number) => {
        const E = solveKepler(o.m0, o.e);
        const cosE = Math.cos(E);
        const sinE = Math.sin(E);
        const r = o.a * (1 - o.e * cosE);

        // position in the perifocal frame
        const px = o.a * (cosE - o.e);
        const py = o.a * Math.sqrt(1 - o.e * o.e) * sinE;

        // velocity in the perifocal frame: differentiate the above, using
        // dE/dt = n / (1 - e cos E) with mean motion n = sqrt(GM/a³)
        const n = Math.sqrt(GM / (o.a * o.a * o.a));
        const edot = n / (1 - o.e * cosE);
        const vpx = -o.a * sinE * edot;
        const vpy = o.a * Math.sqrt(1 - o.e * o.e) * cosE * edot;

        // rotate perifocal -> world: Rz(Ω) · Rx(i) · Rz(ω)
        const co = Math.cos(o.argp), so = Math.sin(o.argp);
        const ci = Math.cos(o.i), si = Math.sin(o.i);
        const cr = Math.cos(o.raan), sr = Math.sin(o.raan);

        const rot = (X: number, Y: number): [number, number, number] => {
          // Rz(ω)
          const x1 = X * co - Y * so;
          const y1 = X * so + Y * co;
          // Rx(i)  — orbital plane tilts about the node line
          const y2 = y1 * ci;
          const z2 = y1 * si;
          // Rz(Ω)
          return [x1 * cr - y2 * sr, x1 * sr + y2 * cr, z2];
        };

        const [wx, wy, wz] = rot(px, py);
        const [wvx, wvy, wvz] = rot(vpx, vpy);
        nx[i] = wx; ny[i] = wy; nz[i] = wz;
        nvx[i] = wvx; nvy[i] = wvy; nvz[i] = wvz;
        void r;
      };

      /** Acceleration under Newton + the first post-Newtonian correction.
       *  `betaFactor` is (1 − β) for dust grains, 1 for the nucleus. */
      const accel = (
        x: number, y: number, z: number,
        vx: number, vy: number, vz: number,
        betaFactor: number,
        out: [number, number, number],
      ) => {
        const r2 = Math.max(1e-6, x * x + y * y + z * z);
        const r = Math.sqrt(r2);
        const inv3 = 1 / (r2 * r);
        const hx = y * vz - z * vy;
        const hy = z * vx - x * vz;
        const hz = x * vy - y * vx;
        const h2 = hx * hx + hy * hy + hz * hz;
        const k = -GM * betaFactor * inv3 * (1 + (3 * h2) / (C2 * r2));
        out[0] = x * k; out[1] = y * k; out[2] = z * k;
      };

      const a3: [number, number, number] = [0, 0, 0];

      for (let i = 0; i < COMETS; i++) {
        stateFromElements(ORBITS[i], i);
        accel(nx[i], ny[i], nz[i], nvx[i], nvy[i], nvz[i], 1, a3);
        nax[i] = a3[0]; nay[i] = a3[1]; naz[i] = a3[2];
      }
      // grains start unborn
      glife.fill(0);
      gage.fill(1);

      const geo = new THREE.BufferGeometry();
      geo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
      geo.setAttribute('aSize', new THREE.BufferAttribute(sizes, 1));
      geo.setAttribute('aAlpha', new THREE.BufferAttribute(alphas, 1));
      geo.setAttribute('aColor', new THREE.BufferAttribute(colors, 3));

      const material = new THREE.ShaderMaterial({
        uniforms: { uPixelRatio: { value: dpr } },
        vertexShader: VERT,
        fragmentShader: FRAG,
        transparent: true,
        depthWrite: false,
        depthTest: false,
        blending: THREE.CustomBlending,
        blendSrc: THREE.SrcAlphaFactor,
        blendDst: THREE.OneFactor,
        blendSrcAlpha: THREE.OneFactor,
        blendDstAlpha: THREE.OneFactor,
      });

      const field = new THREE.Points(geo, material);
      field.frustumCulled = false;
      scene.add(field);

      /** Release one grain from comet `c` into ring slot `slot`. */
      const emit = (c: number, slot: number, ion: boolean) => {
        const g = c * GRAINS + slot;
        // β is the radiation-pressure-to-gravity ratio, set by grain size.
        gbeta[g] = ion ? 1.8 + Math.random() * 1.4 : 0.05 + Math.random() * 0.85;

        // Grains inherit the nucleus state plus a small isotropic ejection
        // velocity from outgassing — that spread is what gives the tail
        // width rather than a hairline.
        const spread = ion ? 0.16 : 0.08;
        gx[g] = nx[c]; gy[g] = ny[c]; gz[g] = nz[c];
        gvx[g] = nvx[c] + (Math.random() - 0.5) * spread;
        gvy[g] = nvy[c] + (Math.random() - 0.5) * spread;
        gvz[g] = nvz[c] + (Math.random() - 0.5) * spread;
        gage[g] = 0;
        glife[g] = ion ? 2.6 + Math.random() * 1.4 : DUST_LIFE_MAX - 2.5 + Math.random() * 2.5;
      };

      const stepNuclei = (dt: number) => {
        for (let c = 0; c < COMETS; c++) {
          /* --- nucleus: velocity Verlet ------------------------- */
          nx[c] += nvx[c] * dt + 0.5 * nax[c] * dt * dt;
          ny[c] += nvy[c] * dt + 0.5 * nay[c] * dt * dt;
          nz[c] += nvz[c] * dt + 0.5 * naz[c] * dt * dt;
          const oax = nax[c], oay = nay[c], oaz = naz[c];
          accel(nx[c], ny[c], nz[c], nvx[c], nvy[c], nvz[c], 1, a3);
          nax[c] = a3[0]; nay[c] = a3[1]; naz[c] = a3[2];
          nvx[c] += 0.5 * (oax + nax[c]) * dt;
          nvy[c] += 0.5 * (oay + nay[c]) * dt;
          nvz[c] += 0.5 * (oaz + naz[c]) * dt;

          /* --- outgassing rate ~ 1/r² --------------------------- */
          const r = Math.max(0.6, Math.hypot(nx[c], ny[c], nz[c]));
          const activity = Math.min(1, 36 / (r * r));
          const perSec = activity * EMIT_PEAK_PER_SEC;
          let n = Math.floor(perSec * dt);
          if (Math.random() < perSec * dt - n) n++;
          for (let k = 0; k < n; k++) {
            const slot = cursor[c] % GRAINS;
            cursor[c] = (cursor[c] + 1) % GRAINS;
            // roughly 40% of released particles are ion-tail material
            emit(c, slot, Math.random() < 0.4);
          }
        }

      };

      /* --- grains: reduced gravity, GM(1 − β) -------------------- */
      const stepGrains = (dt: number) => {
        for (let g = 0; g < G; g++) {
          if (glife[g] <= 0) continue;
          gage[g] += dt;
          if (gage[g] > glife[g]) { glife[g] = 0; continue; }
          accel(gx[g], gy[g], gz[g], gvx[g], gvy[g], gvz[g], 1 - gbeta[g], a3);
          gvx[g] += a3[0] * dt;
          gvy[g] += a3[1] * dt;
          gvz[g] += a3[2] * dt;
          gx[g] += gvx[g] * dt;
          gy[g] += gvy[g] * dt;
          gz[g] += gvz[g] * dt;
        }
      };

      const writeBuffers = () => {
        let k = 0;
        for (let c = 0; c < COMETS; c++) {
          const r = Math.max(0.6, Math.hypot(nx[c], ny[c], nz[c]));
          const activity = Math.min(1, 36 / (r * r));

          // nucleus + coma
          positions[k * 3] = nx[c]; positions[k * 3 + 1] = ny[c]; positions[k * 3 + 2] = nz[c];
          sizes[k] = 3.6 + activity * 7.0;
          alphas[k] = 0.6 + activity * 0.4;
          colors[k * 3] = C_CORE.r; colors[k * 3 + 1] = C_CORE.g; colors[k * 3 + 2] = C_CORE.b;
          k++;

          for (let s = 0; s < GRAINS; s++) {
            const g = c * GRAINS + s;
            if (glife[g] <= 0) {
              alphas[k] = 0;
              sizes[k] = 0;
              positions[k * 3] = 0; positions[k * 3 + 1] = 0; positions[k * 3 + 2] = 0;
              k++;
              continue;
            }
            const life = gage[g] / glife[g];
            const ion = gbeta[g] > 1.5;

            positions[k * 3] = gx[g];
            positions[k * 3 + 1] = gy[g];
            positions[k * 3 + 2] = gz[g];

            if (ion) {
              tmp.copy(C_ION);
              sizes[k] = 2.3 * (1 - life * 0.45);
              alphas[k] = (1 - life) * (1 - life) * 0.40 * (0.4 + activity);
            } else {
              // colour by grain size: low β means a large grain, which is
              // redder and brighter; high β means fine dust, paler
              tmp.copy(C_DUST_BIG).lerp(C_DUST_FINE, Math.min(1, gbeta[g] / 0.9));
              sizes[k] = 2.9 * (1 - life * 0.4);
              alphas[k] = (1 - life) * (1 - life) * 0.44 * (0.4 + activity);
            }
            colors[k * 3] = tmp.r; colors[k * 3 + 1] = tmp.g; colors[k * 3 + 2] = tmp.b;
            k++;
          }
        }
        geo.attributes.position.needsUpdate = true;
        geo.attributes.aSize.needsUpdate = true;
        geo.attributes.aAlpha.needsUpdate = true;
        geo.attributes.aColor.needsUpdate = true;
      };

      const step = (dt: number) => {
        // Nuclei get sub-steps: their eccentric orbits accelerate hard
        // through perihelion and a single step there drifts. Grains stay
        // much further out, so one step is plenty — and that is what makes
        // a population this dense affordable.
        const SUB = 3;
        const h = dt / SUB;
        for (let n = 0; n < SUB; n++) stepNuclei(h);
        stepGrains(dt);
        writeBuffers();
      };

      let onScreen = true;
      const io = new IntersectionObserver(([entry]) => {
        onScreen = entry.isIntersecting;
      });
      io.observe(host);

      const ro = new ResizeObserver(() => {
        const { w, h } = measure();
        renderer.setSize(w, h, false);
        camera.aspect = w / h;
        camera.updateProjectionMatrix();
      });
      ro.observe(host);

      const onLost = (e: Event) => {
        e.preventDefault();
        setFailed(true);
      };
      el.addEventListener('webglcontextlost', onLost);

      // Settle so the first paint shows developed tails rather than four
      // bare nuclei that then grow them.
      for (let i = 0; i < 420; i++) step(1 / 60);
      renderer.render(scene, camera);

      if (!motionOff()) {
        // Capped at ~30fps to match <Singularity>, which shares this page.
        const FRAME_MS = 1000 / 30;
        let last = performance.now();
        let lastDraw = 0;
        const loop = () => {
          raf = requestAnimationFrame(loop);
          if (!onScreen) return;
          const now = performance.now();
          if (now - lastDraw < FRAME_MS) return;
          lastDraw = now;
          const dt = Math.min((now - last) / 1000, 1 / 20); // clamp after a tab switch
          last = now;
          step(dt);
          renderer.render(scene, camera);
        };
        raf = requestAnimationFrame(loop);
      }

      cleanup = () => {
        cancelAnimationFrame(raf);
        io.disconnect();
        ro.disconnect();
        el.removeEventListener('webglcontextlost', onLost);
        geo.dispose();
        material.dispose();
        renderer.dispose();
        if (el.parentNode === host) host.removeChild(el);
      };
    }

    void boot();

    return () => {
      disposed = true;
      cleanup?.();
    };
  }, []);

  if (failed) return null;

  return (
    <div
      ref={hostRef}
      className={className}
      aria-hidden="true"
      style={{ position: 'fixed', inset: 0, zIndex: 0, pointerEvents: 'none' }}
    />
  );
}
