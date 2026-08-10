import { useEffect, useRef, useState } from 'react';
import type * as THREE_NS from 'three';
import { prefersReducedMotion as motionOff } from '@/lib/motion';

/**
 * CometField — physically-modelled comets falling toward an unseen mass.
 *
 * The WebGL backdrop for the login and register screens, and the companion
 * to the landing page's <Singularity>. That one ray-traces null geodesics
 * to show the hole itself; this one integrates *matter* orbits to show what
 * falls into it — the product's own line: secrets fall past the horizon and
 * nothing escapes.
 *
 * ── Orbits ────────────────────────────────────────────────────────────
 * Acceleration carries both terms:
 *
 *     a = −GM·r̂/r²  −  3·GM·h²·r̂/r⁴
 *         └ Newton ┘    └ relativistic correction ┘
 *
 * where h = |r × v| is the specific angular momentum. The second term is
 * the same post-Newtonian correction that bends light on the landing page,
 * and it makes the orbits *precess* — the ellipse slowly rotates instead
 * of closing on itself, which is the Mercury-perihelion effect.
 *
 * Integration is velocity Verlet, not forward Euler. Verlet is symplectic,
 * so orbital energy stays bounded instead of drifting; with Euler these
 * orbits visibly spiral out over a few minutes on an idle login screen.
 *
 * ── Tails ─────────────────────────────────────────────────────────────
 * The defining physical fact about a comet is that its tail points away
 * from the central body, NOT backwards along its path. A trail of past
 * positions is a trajectory, not a tail. So each comet renders two, per
 * real cometary structure:
 *
 *   · ion tail  — gas swept straight anti-radially by the wind, narrow
 *                 and blue-ish.
 *   · dust tail — heavier grains released earlier, which keep more of the
 *                 orbital velocity they were released with and so lag
 *                 behind into a curve, broader and warmer.
 *
 * Outgassing scales as 1/r², so a comet is nearly bare far out and grows
 * a long tail near perihelion — which is exactly when it swings through
 * frame. Brightness follows the same law.
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

const COMETS = 30;
const ION = 34;  // particles per ion tail — must be dense enough that
const DUST = 34; // adjacent points overlap, or the tail reads as dashes
const PER = 1 + ION + DUST; // + nucleus
const POINTS = COMETS * PER;

/* Geometric-ish units. Camera at CAM_Z with a 50deg vertical fov gives a
   visible half-height of ~0.466 * CAM_Z at the origin; the spawn ring is
   kept just inside that so comets enter frame rather than off-screen. */
const CAM_Z = 34;
const GM = 90;
/** Speed of light in these units. The relativistic term below is a
 *  correction of order (v/c)^2, so c MUST be well above orbital speed
 *  (~2.5 here) or the "correction" dominates the Newtonian force and the
 *  orbits become nonsense. c = 30 puts the precession around 1% per orbit:
 *  visible over time, never destabilising. */
const C2 = 900;
const R_SPAWN = 15;
const R_CAPTURE = 1.1;
const R_ESCAPE = 34;
/** Outgassing reference radius: tails are ~full length inside this. */
const R_ACTIVE = 7.5;

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

      /* --- state ------------------------------------------------- */
      const pos = new Float32Array(COMETS * 3);
      const vel = new Float32Array(COMETS * 3);
      const acc = new Float32Array(COMETS * 3);

      const positions = new Float32Array(POINTS * 3);
      const sizes = new Float32Array(POINTS);
      const alphas = new Float32Array(POINTS);
      const colors = new Float32Array(POINTS * 3);

      // nucleus / coma, ion gas, dust grains
      const C_CORE = new THREE.Color('#ffffff');
      const C_ION = new THREE.Color('#8fd4ff');
      const C_DUST = new THREE.Color('#ffc98a');
      const C_HALO = new THREE.Color('#c9b6ff');

      /** a = -(GM/r^3)·r·(1 + 3h²/(c²r²))  — Newton plus the first
       *  post-Newtonian correction, which precesses the orbit. */
      const accelInto = (i: number, out: Float32Array) => {
        const x = pos[i * 3], y = pos[i * 3 + 1], z = pos[i * 3 + 2];
        const vx = vel[i * 3], vy = vel[i * 3 + 1], vz = vel[i * 3 + 2];
        const r2 = x * x + y * y + z * z;
        const r = Math.sqrt(r2);
        const inv3 = 1 / (r2 * r);

        // specific angular momentum h = r x v
        const hx = y * vz - z * vy;
        const hy = z * vx - x * vz;
        const hz = x * vy - y * vx;
        const h2 = hx * hx + hy * hy + hz * hz;

        // Newtonian, plus a first-order relativistic precession term.
        // The (c² r²) denominator is what keeps it a *correction*.
        const k = -GM * inv3 * (1 + (3 * h2) / (C2 * r2));
        out[i * 3] = x * k;
        out[i * 3 + 1] = y * k;
        out[i * 3 + 2] = z * k;
      };

      /**
       * @param anywhere seed at a random orbital phase instead of on the
       *   outer ring. Used once at start-up: spawning every comet at the
       *   same radius leaves them phase-locked, and 30 synchronised tails
       *   all pointing outward read as a firework rather than a comet
       *   field. During the run, respawns use the ring so comets enter
       *   frame from outside.
       */
      const spawn = (i: number, anywhere = false) => {
        const a = Math.random() * Math.PI * 2;
        const r = anywhere
          ? 2.8 + Math.random() * (R_SPAWN * 1.35 - 2.8)
          : R_SPAWN * (0.9 + Math.random() * 0.45);
        const x = Math.cos(a) * r;
        const y = Math.sin(a) * r;
        const z = (Math.random() - 0.5) * 6;

        // Sub-circular tangential speed with an inward component gives the
        // eccentric, perihelion-passing orbits that grow tails in frame.
        const vc = Math.sqrt(GM / r);
        const tangential = vc * (0.5 + Math.random() * 0.42);
        // at start-up some comets are already outbound past perihelion
        const dir = anywhere && Math.random() < 0.45 ? -1 : 1;
        const inward = dir * vc * (0.14 + Math.random() * 0.2);

        pos[i * 3] = x; pos[i * 3 + 1] = y; pos[i * 3 + 2] = z;
        vel[i * 3] = -Math.sin(a) * tangential - (x / r) * inward;
        vel[i * 3 + 1] = Math.cos(a) * tangential - (y / r) * inward;
        vel[i * 3 + 2] = (Math.random() - 0.5) * 0.04;
        accelInto(i, acc);
      };

      for (let i = 0; i < COMETS; i++) spawn(i, true);

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

      /* --- velocity Verlet --------------------------------------- */
      const integrate = (dt: number) => {
        for (let i = 0; i < COMETS; i++) {
          const b = i * 3;
          // x(t+dt) = x + v dt + a dt^2 / 2
          pos[b] += vel[b] * dt + 0.5 * acc[b] * dt * dt;
          pos[b + 1] += vel[b + 1] * dt + 0.5 * acc[b + 1] * dt * dt;
          pos[b + 2] += vel[b + 2] * dt + 0.5 * acc[b + 2] * dt * dt;

          const ax = acc[b], ay = acc[b + 1], az = acc[b + 2];
          accelInto(i, acc);
          // v(t+dt) = v + (a + a_new) dt / 2
          vel[b] += 0.5 * (ax + acc[b]) * dt;
          vel[b + 1] += 0.5 * (ay + acc[b + 1]) * dt;
          vel[b + 2] += 0.5 * (az + acc[b + 2]) * dt;

          const r = Math.hypot(pos[b], pos[b + 1], pos[b + 2]);
          if (r < R_CAPTURE || r > R_ESCAPE) spawn(i);
        }
      };

      /* --- build the two tails ----------------------------------- */
      const buildTails = () => {
        for (let i = 0; i < COMETS; i++) {
          const b = i * 3;
          const x = pos[b], y = pos[b + 1], z = pos[b + 2];
          const r = Math.max(0.001, Math.hypot(x, y, z));
          const rx = x / r, ry = y / r, rz = z / r; // anti-radial = away from the mass

          const vx = vel[b], vy = vel[b + 1], vz = vel[b + 2];
          const vm = Math.max(0.001, Math.hypot(vx, vy, vz));
          const tx = vx / vm, ty = vy / vm, tz = vz / vm;

          // outgassing ~ 1/r^2, saturating near perihelion
          const q = Math.min(1, (R_ACTIVE * R_ACTIVE) / (r * r));
          const ionLen = 1.2 + q * 7.5;
          const dustLen = 0.9 + q * 4.6;
          const bright = 0.16 + q * 0.9;

          let k = i * PER;

          // nucleus + coma
          positions[k * 3] = x; positions[k * 3 + 1] = y; positions[k * 3 + 2] = z;
          sizes[k] = 2.4 + q * 6.0;
          alphas[k] = Math.min(1, 0.62 + bright);
          colors[k * 3] = C_CORE.r; colors[k * 3 + 1] = C_CORE.g; colors[k * 3 + 2] = C_CORE.b;
          k++;

          // ion tail — straight, anti-radial
          for (let t = 0; t < ION; t++) {
            const s = (t + 1) / ION;
            const d = s * ionLen;
            positions[k * 3] = x + rx * d;
            positions[k * 3 + 1] = y + ry * d;
            positions[k * 3 + 2] = z + rz * d;
            sizes[k] = (2.9 - s * 1.95) * 1.55;
            alphas[k] = Math.pow(1 - s, 1.5) * bright * 1.15;
            const c = s < 0.35 ? C_HALO : C_ION;
            colors[k * 3] = c.r; colors[k * 3 + 1] = c.g; colors[k * 3 + 2] = c.b;
            k++;
          }

          // dust tail — anti-radial but lagging behind the orbital motion,
          // so it curves. Grains released earlier retain more of the
          // velocity they had then, hence the s^1.6 lag term.
          for (let t = 0; t < DUST; t++) {
            const s = (t + 1) / DUST;
            const d = s * dustLen;
            const lag = Math.pow(s, 1.6) * dustLen * 0.85;
            positions[k * 3] = x + rx * d - tx * lag;
            positions[k * 3 + 1] = y + ry * d - ty * lag;
            positions[k * 3 + 2] = z + rz * d - tz * lag;
            sizes[k] = (3.1 - s * 2.1) * 1.75;
            alphas[k] = Math.pow(1 - s, 1.5) * bright * 0.62;
            colors[k * 3] = C_DUST.r; colors[k * 3 + 1] = C_DUST.g; colors[k * 3 + 2] = C_DUST.b;
            k++;
          }
        }
        geo.attributes.position.needsUpdate = true;
        geo.attributes.aSize.needsUpdate = true;
        geo.attributes.aAlpha.needsUpdate = true;
        geo.attributes.aColor.needsUpdate = true;
      };

      const step = (dt: number) => {
        // fixed sub-steps keep the integrator stable through perihelion,
        // where the acceleration spikes
        const SUB = 3;
        const h = dt / SUB;
        for (let n = 0; n < SUB; n++) integrate(h);
        buildTails();
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

      // Settle so the first paint shows an evolved field with real tails,
      // not a ring of bare nuclei that then springs to life.
      for (let i = 0; i < 260; i++) step(1 / 60);
      renderer.render(scene, camera);

      if (!motionOff()) {
        // Capped at ~30fps to match <Singularity>, which shares the page on
        // the auth screens. Two uncapped WebGL loops drop integrated
        // graphics to single-digit frame rates.
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
