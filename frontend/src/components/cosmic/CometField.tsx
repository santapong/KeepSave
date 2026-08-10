import { useEffect, useRef, useState } from 'react';
import type * as THREE_NS from 'three';
import { prefersReducedMotion as motionOff } from '@/lib/motion';

/**
 * CometField — the WebGL backdrop for the login and register screens.
 *
 * Companion piece to <Singularity> on the landing page. The landing shows
 * the hole itself; this shows what falls into it, which is the product's
 * own line: secrets fall past the horizon and nothing escapes.
 *
 * Physics note: unlike the landing's geodesic tracer, these are *matter*
 * trajectories, so plain Newtonian gravity is the correct model —
 * a = −GM·r̂/r². Comets are launched on the outer ring with a tangential
 * component, which yields the hyperbolic and highly eccentric flybys that
 * whip around the mass and slingshot away. Anything that falls inside the
 * capture radius is respawned on the rim.
 *
 * Rendering: one THREE.Points system holds every comet's trail. Each trail
 * point carries its own size and alpha, tapering head → tail, which gives
 * a soft glowing streak. Deliberately not LineSegments: WebGL ignores
 * `linewidth`, so lines would render as hard 1px hairlines.
 *
 * Degradations:
 *   · `prefers-reduced-motion` / `data-motion="off"` -> one static frame
 *     of an already-evolved field, then nothing moves.
 *   · no WebGL / three fails to load -> renders nothing. The CSS
 *     <Starfield> behind it is the backdrop, so the screen still reads.
 *   · scrolled out of view -> the rAF loop parks itself.
 */

interface CometFieldProps {
  className?: string;
}

const COMETS = 38;
const TRAIL = 22;
const POINTS = COMETS * TRAIL;

/* Scale note: the camera sits at z = CAM_Z with a 50 deg vertical fov, so
   the visible half-height at the origin is CAM_Z * tan(25 deg) ~= 0.466 *
   CAM_Z. R_SPAWN is kept just inside that so comets are launched on the
   edge of frame rather than far outside it, which otherwise leaves the
   field looking almost empty. */
const CAM_Z = 34;
const GM = 90;        // gravitational parameter — also sets speed, see TRAIL note
const R_SPAWN = 15;   // launch ring
const R_CAPTURE = 1.2;
const R_ESCAPE = 34;

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
          // this (and the alpha-accumulating blend below) the whole field
          // composites at alpha 0 and nothing is visible.
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

      /* --- comet state ------------------------------------------- */
      const px = new Float32Array(COMETS);
      const py = new Float32Array(COMETS);
      const pz = new Float32Array(COMETS);
      const vx = new Float32Array(COMETS);
      const vy = new Float32Array(COMETS);
      const vz = new Float32Array(COMETS);
      // ring buffer of past positions, newest at index 0
      const trail = new Float32Array(COMETS * TRAIL * 3);

      const positions = new Float32Array(POINTS * 3);
      const sizes = new Float32Array(POINTS);
      const alphas = new Float32Array(POINTS);
      const colors = new Float32Array(POINTS * 3);

      const PALETTE = [
        new THREE.Color('#c9b6ff'), // periwinkle
        new THREE.Color('#ff7ab8'), // flare magenta
        new THREE.Color('#ffd9a0'), // hot gold
        new THREE.Color('#9fe8f5'), // photon cyan
      ];

      const spawn = (i: number, seedTrail: boolean) => {
        const a = Math.random() * Math.PI * 2;
        const r = R_SPAWN * (0.85 + Math.random() * 0.5);
        const x = Math.cos(a) * r;
        const y = Math.sin(a) * r;
        const z = (Math.random() - 0.5) * 7;

        // circular speed at this radius, scaled below 1 so the orbit
        // decays inward instead of closing — that is what makes them fall.
        const vc = Math.sqrt(GM / r);
        const tangential = vc * (0.55 + Math.random() * 0.5);
        const inward = vc * (0.12 + Math.random() * 0.22);

        px[i] = x; py[i] = y; pz[i] = z;
        vx[i] = -Math.sin(a) * tangential - (x / r) * inward;
        vy[i] = Math.cos(a) * tangential - (y / r) * inward;
        vz[i] = (Math.random() - 0.5) * 0.05;

        const c = PALETTE[(Math.random() * PALETTE.length) | 0];
        for (let t = 0; t < TRAIL; t++) {
          const k = i * TRAIL + t;
          colors[k * 3] = c.r; colors[k * 3 + 1] = c.g; colors[k * 3 + 2] = c.b;
          const f = 1 - t / TRAIL;
          sizes[k] = 0.30 + f * f * 4.0;
          alphas[k] = 0.05 + f * f * 0.68;
          if (seedTrail) {
            trail[(i * TRAIL + t) * 3] = x;
            trail[(i * TRAIL + t) * 3 + 1] = y;
            trail[(i * TRAIL + t) * 3 + 2] = z;
          }
        }
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

      const step = (dt: number) => {
        for (let i = 0; i < COMETS; i++) {
          const x = px[i], y = py[i], z = pz[i];
          const r2 = x * x + y * y + z * z;
          const r = Math.sqrt(r2);

          if (r < R_CAPTURE || r > R_ESCAPE) {
            spawn(i, true);
            continue;
          }

          // Newtonian attraction toward the origin
          const a = -GM / (r2 * r);
          vx[i] += x * a * dt;
          vy[i] += y * a * dt;
          vz[i] += z * a * dt;

          px[i] += vx[i] * dt;
          py[i] += vy[i] * dt;
          pz[i] += vz[i] * dt;

          // shift the trail back one slot, newest first
          const base = i * TRAIL * 3;
          for (let t = TRAIL - 1; t > 0; t--) {
            trail[base + t * 3] = trail[base + (t - 1) * 3];
            trail[base + t * 3 + 1] = trail[base + (t - 1) * 3 + 1];
            trail[base + t * 3 + 2] = trail[base + (t - 1) * 3 + 2];
          }
          trail[base] = px[i];
          trail[base + 1] = py[i];
          trail[base + 2] = pz[i];
        }
        positions.set(trail);
        geo.attributes.position.needsUpdate = true;
        geo.attributes.aSize.needsUpdate = true;
        geo.attributes.aAlpha.needsUpdate = true;
        geo.attributes.aColor.needsUpdate = true;
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

      // Settle the field before the first paint so it never appears as a
      // ring of dots that then springs to life.
      for (let i = 0; i < 220; i++) step(1 / 60);
      renderer.render(scene, camera);

      if (!motionOff()) {
        let last = performance.now();
        const loop = () => {
          raf = requestAnimationFrame(loop);
          const now = performance.now();
          const dt = Math.min((now - last) / 1000, 1 / 20); // clamp after tab-switch
          last = now;
          if (!onScreen) return;
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
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 0,
        pointerEvents: 'none',
      }}
    />
  );
}
