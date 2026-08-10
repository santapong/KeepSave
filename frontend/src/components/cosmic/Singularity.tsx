import { useEffect, useRef, useState } from 'react';
import type * as THREE_NS from 'three';
import { EventHorizon } from './EventHorizon';
import { prefersReducedMotion } from '@/lib/motion';

/**
 * Singularity — a ray-traced Schwarzschild black hole.
 *
 * The previous version faked lensing by drawing a second copy of the disk
 * mirrored over the top. This one does the real thing: every pixel fires a
 * ray and integrates a null geodesic through curved spacetime, so the
 * lensing, the photon ring and the Einstein ring fall out of the physics
 * instead of being drawn on.
 *
 * Units are geometric with the Schwarzschild radius rs = 1, so:
 *   r = 1.0   event horizon
 *   r = 1.5   photon sphere (where light can orbit)
 *   r = 3.0   ISCO — innermost stable circular orbit, and therefore the
 *             inner edge of the accretion disk
 *
 * The integrator is the Cartesian form of the standard orbit equation
 *   d²u/dφ² + u = 3Mu²      (u = 1/r)
 * rewritten as an acceleration  a = −3/2 · h² · r̂ / r⁵, where h is the
 * ray's conserved specific angular momentum. That one term is what bends
 * light correctly: it gives the 1.75″ solar deflection in the weak field
 * and a photon sphere in the strong field.
 *
 * Disk shading is physical too:
 *   · Keplerian orbital velocity  v = √(M/r)
 *   · relativistic Doppler beaming — the approaching limb goes bright and
 *     blue, the receding one dim and red. This asymmetry is the single
 *     most recognisable feature of a real black hole image.
 *   · gravitational redshift  √(1 − rs/r) reddening the inner annulus
 *   · intensity ∝ g³, the standard beaming exponent
 *
 * Escaped rays sample a procedural sky, so background stars get lensed
 * into an Einstein ring around the shadow.
 *
 * Degradations, in order:
 *   · `prefers-reduced-motion` / `data-motion="off"` -> one static frame.
 *   · no WebGL / context lost / three fails to load -> the CSS
 *     <EventHorizon>, so the hero is never empty.
 *   · scrolled out of view -> the rAF loop parks itself.
 *
 * Cost control: the march is fill-rate bound, so the drawing buffer is
 * rendered at RESOLUTION_SCALE and upscaled by CSS. The subject is all
 * glow and gradient, so the softness costs nothing visually.
 */

interface SingularityProps {
  /** Square size for the CSS <EventHorizon> fallback only. */
  size?: number;
  /**
   * Fraction of CSS pixels actually rendered, upscaled by CSS. The march is
   * fill-rate bound and the subject is all glow, so the softness is free.
   * Lower it where a second WebGL context shares the page — the auth
   * screens also run <CometField>.
   */
  resolutionScale?: number;
  className?: string;
}

const DEFAULT_RESOLUTION_SCALE = 0.55;

const QUAD_VERT = /* glsl */ `
  void main() {
    gl_Position = vec4(position.xy, 0.0, 1.0);
  }
`;

const LENS_FRAG = /* glsl */ `
  precision highp float;

  uniform vec2  uRes;
  uniform float uTime;
  uniform vec3  uCam;        // camera position, geometric units
  uniform mat3  uBasis;      // right / up / forward
  uniform float uDiskIn;     // ISCO
  uniform float uDiskOut;
  uniform vec3  uHot;        // inner annulus, gold-white
  uniform vec3  uFlare;      // mid, magenta
  uniform vec3  uPlasma;     // outer, violet
  uniform float uExposure;

  const int   STEPS   = 110;
  const float HORIZON = 1.0;   // rs
  const float M       = 0.5;   // rs = 2M
  const float ESCAPE  = 60.0;
  /* Rays whose closest approach to the mass exceeds this are neither
     lensed appreciably nor able to reach the disk (outer edge 9), so they
     skip the march entirely and just sample the sky. Most of the frame is
     empty sky, which makes this the single biggest saving available. */
  const float B_MAX   = 17.0;

  float hash21(vec2 p) {
    p = fract(p * vec2(123.34, 456.21));
    p += dot(p, p + 45.32);
    return fract(p.x * p.y);
  }
  float vnoise(vec2 p) {
    vec2 i = floor(p), f = fract(p);
    vec2 u = f * f * (3.0 - 2.0 * f);
    return mix(mix(hash21(i), hash21(i + vec2(1.0, 0.0)), u.x),
               mix(hash21(i + vec2(0.0, 1.0)), hash21(i + vec2(1.0, 1.0)), u.x), u.y);
  }
  float fbm(vec2 p) {
    float v = 0.0, a = 0.5;
    for (int i = 0; i < 5; i++) { v += a * vnoise(p); p *= 2.02; a *= 0.5; }
    return v;
  }

  /* Procedural sky, sampled with the ray's FINAL direction — so the
     lensing bends the star field into the Einstein ring for free. */
  vec3 sky(vec3 rd) {
    vec2 sph = vec2(atan(rd.z, rd.x), acos(clamp(rd.y, -1.0, 1.0)));
    vec3 acc = vec3(0.0);
    for (int layer = 0; layer < 2; layer++) {
      float scale = layer == 0 ? 46.0 : 92.0;
      vec2 g  = sph * scale;
      vec2 id = floor(g);
      vec2 f  = fract(g) - 0.5;
      float h = hash21(id + float(layer) * 37.0);
      if (h > 0.955) {
        float d = length(f);
        float bright = pow(max(0.0, 1.0 - d * 2.6), 8.0) * (h - 0.955) * 24.0;
        vec3 tint = mix(vec3(0.72, 0.78, 1.0), vec3(1.0, 0.92, 0.82), hash21(id * 1.7));
        acc += tint * bright;
      }
    }
    return acc;
  }

  /* Emission from the accretion disk at an equatorial crossing. */
  vec3 diskEmission(vec3 hit, vec3 vel) {
    float r   = length(hit.xz);
    float t   = clamp((r - uDiskIn) / (uDiskOut - uDiskIn), 0.0, 1.0);
    float phi = atan(hit.z, hit.x);

    // Keplerian angular velocity: Omega ~ r^-3/2, so inner annuli lap
    // the outer ones and the disk shears itself into streaks.
    float orbit   = uTime * 2.6 / pow(r, 1.5);
    float phiFlow = phi + orbit;   // co-rotating azimuth
    float turb  = fbm(vec2(phiFlow * 3.4, r * 1.1));
    turb        = mix(0.30, 1.55, turb);
    turb       *= 0.72 + 0.5 * fbm(vec2(r * 6.5, phiFlow * 8.0));
    // bright filaments dragged around with the flow
    turb       *= 0.85 + 0.35 * fbm(vec2(phiFlow * 1.7, r * 2.6));

    /* --- relativistic beaming ------------------------------------ */
    vec3  tangent = normalize(vec3(-hit.z, 0.0, hit.x));  // orbital direction
    float speed   = sqrt(M / r);                           // Keplerian
    vec3  beta    = tangent * speed;
    vec3  toEye   = normalize(-vel);
    float gamma   = 1.0 / sqrt(max(1e-4, 1.0 - speed * speed));
    float doppler = 1.0 / max(0.05, gamma * (1.0 - dot(beta, toEye)));

    // gravitational redshift climbing out of the well
    float grav = sqrt(max(0.0, 1.0 - HORIZON / r));
    float g    = doppler * grav;

    /* --- temperature ramp ---------------------------------------- */
    vec3 col = mix(uHot, uFlare, smoothstep(0.0, 0.42, t));
    col      = mix(col, uPlasma, smoothstep(0.40, 1.0, t));
    col     += uHot * (1.0 - smoothstep(0.0, 0.18, t)) * 0.85;

    // blueshift the boosted limb, redden the receding one
    col = mix(col, col * vec3(0.72, 0.85, 1.25), clamp((g - 1.0) * 0.7, 0.0, 1.0));
    col = mix(col, col * vec3(1.25, 0.70, 0.45), clamp((1.0 - g) * 0.9, 0.0, 1.0));

    // soft radial falloff so neither edge is a hard rim
    float edge = smoothstep(0.0, 0.06, t) * (1.0 - smoothstep(0.55, 1.0, t));

    return col * turb * edge * pow(g, 3.0);
  }

  void main() {
    vec2 uv = (gl_FragCoord.xy - 0.5 * uRes) / uRes.y;

    vec3 pos = uCam;
    // uBasis' third column is the true forward vector (camera -> origin),
    // not the usual OpenGL -Z, so the depth term is POSITIVE here.
    // Negating it fires every ray away from the hole into empty sky.
    vec3 vel = normalize(uBasis * vec3(uv, 1.1));

    // conserved specific angular momentum of this ray
    vec3  hvec = cross(pos, vel);
    float h2   = dot(hvec, hvec);

    // |r x v| with v normalised IS the impact parameter. Bail out early for
    // rays that pass wide of the mass, and for any ray already heading away
    // from it — neither can be deflected into the disk or the shadow.
    float b = sqrt(h2);
    if (b > B_MAX || dot(pos, vel) > 0.0) {
      vec3 far = sky(normalize(vel));
      float farLum = dot(far, vec3(0.2126, 0.7152, 0.0722));
      far = far / (far + vec3(0.85));
      gl_FragColor = vec4(pow(max(far, 0.0), vec3(0.85)), clamp(farLum * 1.5, 0.0, 1.0));
      return;
    }

    vec3  acc      = vec3(0.0);
    float prevY    = pos.y;
    bool  captured = false;

    for (int i = 0; i < STEPS; i++) {
      float r2 = dot(pos, pos);
      float r  = sqrt(r2);
      if (r < HORIZON) { captured = true; break; }
      if (r > ESCAPE)  break;

      // finer steps deep in the well, where curvature is strongest
      float dt = clamp((r - HORIZON) * 0.16 + 0.02, 0.015, 1.1);

      // the whole of general relativity, for our purposes
      vec3 a = -1.5 * h2 * pos / (r2 * r2 * r);

      vec3 npos = pos + vel * dt + 0.5 * a * dt * dt;
      vel += a * dt;

      // did we cross the equatorial plane inside the disk annulus?
      if (prevY * npos.y < 0.0) {
        float k   = prevY / (prevY - npos.y);
        vec3  hit = mix(pos, npos, k);
        float rr  = length(hit.xz);
        if (rr > uDiskIn && rr < uDiskOut) {
          acc += diskEmission(hit, vel);
        }
      }

      prevY = npos.y;
      pos   = npos;
    }

    vec3  col = acc * uExposure;
    float alpha;

    if (captured) {
      // The shadow is genuinely opaque — it occludes the page behind it,
      // which is what sells it as a hole rather than a decal.
      col   = vec3(0.0);
      alpha = 1.0;
    } else {
      col += sky(normalize(vel));
      // Empty sky stays transparent so the page's own starfield reads
      // through; only emitted light composites.
      float lum = dot(col, vec3(0.2126, 0.7152, 0.0722));
      alpha = clamp(lum * 1.5, 0.0, 1.0);
    }

    // filmic-ish rolloff so the boosted limb doesn't clip to flat white
    col = col / (col + vec3(0.85));
    col = pow(max(col, 0.0), vec3(0.85));

    gl_FragColor = vec4(col, alpha);
  }
`;


export function Singularity({
  size = 560,
  resolutionScale = DEFAULT_RESOLUTION_SCALE,
  className,
}: SingularityProps) {
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
        // premultipliedAlpha:false so the shader's own (colour, alpha) pair
        // composites correctly over the page: opaque black for the shadow,
        // glow for the disk, clear for empty sky.
        renderer = new THREE.WebGLRenderer({
          alpha: true,
          antialias: false, // pointless for a full-screen raymarch
          premultipliedAlpha: false,
          powerPreference: 'high-performance',
        });
      } catch {
        setFailed(true);
        return;
      }

      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      renderer.setPixelRatio(dpr * resolutionScale);
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
      const camera = new THREE.OrthographicCamera(-1, 1, 1, -1, 0, 1);

      const uniforms = {
        uRes: { value: new THREE.Vector2(first.w, first.h) },
        uTime: { value: 0 },
        uCam: { value: new THREE.Vector3() },
        uBasis: { value: new THREE.Matrix3() },
        uDiskIn: { value: 3.0 }, // ISCO
        uDiskOut: { value: 9.0 },
        uHot: { value: new THREE.Color('#ffd9a0') },
        uFlare: { value: new THREE.Color('#ff5fa8') },
        uPlasma: { value: new THREE.Color('#8b5cf6') },
        uExposure: { value: 1.9 },
      };

      const material = new THREE.ShaderMaterial({
        uniforms,
        vertexShader: QUAD_VERT,
        fragmentShader: LENS_FRAG,
        transparent: true,
        depthWrite: false,
        depthTest: false,
      });
      const quad = new THREE.Mesh(new THREE.PlaneGeometry(2, 2), material);
      quad.frustumCulled = false;
      scene.add(quad);

      const syncResolution = () => {
        const buf = new THREE.Vector2();
        renderer.getDrawingBufferSize(buf);
        uniforms.uRes.value.set(buf.x, buf.y);
      };
      syncResolution();

      /* --- camera orbit ---------------------------------------------
         Kept nearly edge-on: a shallow inclination is what makes the far
         side of the disk arc up over the shadow. */
      const RADIUS = 25;
      let yaw = 0;
      let pitch = 0.115; // radians above the disk plane
      let targetYaw = 0;
      let targetPitch = 0.115;

      const onPointer = (e: PointerEvent) => {
        const r = host.getBoundingClientRect();
        targetYaw = ((e.clientX - r.left) / r.width - 0.5) * 0.5;
        targetPitch = 0.115 + ((e.clientY - r.top) / r.height - 0.5) * 0.22;
      };
      window.addEventListener('pointermove', onPointer, { passive: true });

      const right = new THREE.Vector3();
      const up = new THREE.Vector3();
      const forward = new THREE.Vector3();
      const worldUp = new THREE.Vector3(0, 1, 0);

      const draw = (t: number) => {
        // The host can legitimately have no size — on the auth screens the
        // aside is display:none below 1100px. Marching a 0x0 buffer is pure
        // waste, so skip until the ResizeObserver reports real dimensions.
        if (uniforms.uRes.value.x < 1 || uniforms.uRes.value.y < 1) return;

        uniforms.uTime.value = t;

        yaw += (targetYaw - yaw) * 0.045;
        pitch += (targetPitch - pitch) * 0.045;
        const inclination = Math.max(0.02, Math.min(0.42, pitch));
        const angle = yaw + t * 0.018;

        uniforms.uCam.value.set(
          Math.cos(angle) * Math.cos(inclination) * RADIUS,
          Math.sin(inclination) * RADIUS,
          Math.sin(angle) * Math.cos(inclination) * RADIUS,
        );

        forward.copy(uniforms.uCam.value).negate().normalize();
        right.crossVectors(forward, worldUp).normalize();
        up.crossVectors(right, forward).normalize();
        // three.js Matrix3.set is row-major; we want the columns to be
        // right / up / forward so that basis * vec3(uv, -f) builds the ray.
        uniforms.uBasis.value.set(
          right.x, up.x, forward.x,
          right.y, up.y, forward.y,
          right.z, up.z, forward.z,
        );

        renderer.render(scene, camera);
      };

      let onScreen = true;
      const io = new IntersectionObserver(([entry]) => {
        onScreen = entry.isIntersecting;
      });
      io.observe(host);

      const ro = new ResizeObserver(() => {
        const { w, h } = measure();
        const wasHidden = uniforms.uRes.value.x < 1 || uniforms.uRes.value.y < 1;
        renderer.setSize(w, h, false);
        // No camera aspect to update: this is a fullscreen quad on an
        // orthographic camera, and the shader derives aspect from uRes.
        syncResolution();
        // Became visible (breakpoint crossed): paint immediately rather than
        // waiting for the next frame, which never comes under reduced motion.
        if (wasHidden) draw(3.2);
      });
      ro.observe(host);

      const onLost = (e: Event) => {
        e.preventDefault();
        setFailed(true);
      };
      el.addEventListener('webglcontextlost', onLost);

      const still = prefersReducedMotion();
      const started = performance.now();

      // One frame up front so the hero is never empty.
      draw(3.2);

      if (!still) {
        // Cap at ~30fps. The disk turns slowly enough that 30 and 60 are
        // indistinguishable, and this halves GPU cost — which matters:
        // the march runs at 15fps uncapped on Intel integrated graphics.
        const FRAME_MS = 1000 / 30;
        let lastDraw = 0;
        const loop = () => {
          raf = requestAnimationFrame(loop);
          if (!onScreen) return;
          const now = performance.now();
          if (now - lastDraw < FRAME_MS) return;
          lastDraw = now;
          draw((now - started) / 1000);
        };
        raf = requestAnimationFrame(loop);
      }

      cleanup = () => {
        cancelAnimationFrame(raf);
        io.disconnect();
        ro.disconnect();
        window.removeEventListener('pointermove', onPointer);
        el.removeEventListener('webglcontextlost', onLost);
        quad.geometry.dispose();
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
  }, [resolutionScale]);

  if (failed) return <EventHorizon size={size} className={className} />;

  return (
    <div
      ref={hostRef}
      className={className}
      aria-hidden="true"
      style={{ width: '100%', height: '100%', pointerEvents: 'none' }}
    />
  );
}
