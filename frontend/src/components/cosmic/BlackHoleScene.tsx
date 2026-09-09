import { useEffect, useRef, useState } from 'react';
import { useMotion } from '../../hooks/useMotion';
import { EventHorizon } from './EventHorizon';
import { MotionToggle } from './MotionToggle';

// Schwarzschild light paths are integrated once and sampled at disk-plane
// crossings. Appearance (temperature palette/turbulence/exposure) is art-directed.
const fragmentShader = `
precision highp float;
varying vec2 vUv;
uniform float phase;
uniform float aspect;
uniform sampler2D rayTable;
const float PI = 3.141592653589793;
const vec2 TABLE_SIZE = vec2(1024., 512.);

// Manual bilinear interpolation works without float-linear-filter extensions.
float inverseRadius(float b, float phi) {
  float critical = sqrt(27.);
  float coordinate = b < critical
    ? .4 * (1. - sqrt(max(0., 1. - b / critical)))
    : .4 + .6 * sqrt(max(0., (b - critical) / (32. - critical)));
  vec2 index = clamp(vec2(coordinate, phi / (3. * PI)), 0., 1.) * (TABLE_SIZE - 1.);
  vec2 base = floor(index), f = fract(index);
  vec2 uv = (base + .5) / TABLE_SIZE;
  vec2 d = 1. / TABLE_SIZE;
  float a = texture2D(rayTable, uv).r;
  float c = texture2D(rayTable, uv + vec2(0., d.y)).r;
  float e = texture2D(rayTable, uv + vec2(d.x, 0.)).r;
  float g = texture2D(rayTable, uv + d).r;
  return mix(mix(a, e, f.x), mix(c, g, f.x), f.y);
}

vec3 diskLight(float radius, float azimuth, float angularMomentum) {
  // Thin disk outside the Schwarzschild ISCO (6M), smoothly tapering at 18M.
  float edge = smoothstep(6., 6.6, radius) * (1. - smoothstep(15., 18., radius));
  float omega = pow(radius, -1.5);
  // Circular emitter redshift: gravitational + transverse + longitudinal Doppler.
  float shift = sqrt(max(.01, 1. - 3. / radius)) / max(.3, 1. + omega * angularMomentum);
  float temperature = pow(6. / radius, .75) * pow(max(.001, 1. - sqrt(6. / radius)), .25);
  float observedTemperature = temperature * shift;
  vec3 warm = mix(vec3(.62, .11, .018), vec3(1., .49, .14), smoothstep(.13, .32, observedTemperature));
  warm = mix(warm, vec3(1., .89, .64), smoothstep(.32, .53, observedTemperature));
  // Differential orbital motion; integer frequency bands keep the loop seamless.
  float rate = floor(38. * pow(6. / radius, 1.5) + .5);
  float flow = azimuth - phase * rate;
  float bands = .78 + .10 * sin(radius * 6. + sin(flow * 3.) * 1.5)
                        + .05 * sin(radius * 19. - flow * 6.)
                        + .025 * sin(radius * 43. + flow * 11.);
  float brightness = 2.8 * pow(shift, 3.) * pow(7. / radius, 1.6);
  return warm * edge * bands * brightness;
}

void main() {
  vec2 p = (vUv - .5) * vec2(aspect, 1.) * 39.;
  // A steady, nearly edge-on plane: the disk flows, the horizon does not wobble.
  float roll = -.065;
  p = mat2(cos(roll), -sin(roll), sin(roll), cos(roll)) * p;
  float impact = length(p);
  if (impact > 28.) { gl_FragColor = vec4(0.); return; }
  vec2 direction = p / max(impact, .001);
  float inclination = 1.43; // 82 degrees from the disk normal.
  vec3 eye = vec3(0., cos(inclination), sin(inclination));
  vec3 tangent = vec3(direction.x, direction.y * sin(inclination), -direction.y * cos(inclination));
  float firstCrossing = atan(-eye.y, tangent.y) + PI;
  vec3 radiance = vec3(0.);
  float coverage = 0.;
  // Up to three ordered disk intersections: direct, secondary, tertiary images.
  // An opaque foreground disk blocks farther images along the same ray.
  for (int imageIndex = 0; imageIndex < 3; imageIndex++) {
    float phi = firstCrossing + float(imageIndex) * PI;
    float u = inverseRadius(impact, phi);
    float radius = 1. / max(u, .00001);
    if (radius > 6. && radius < 18. && coverage < .99) {
      vec3 position = eye * cos(phi) + tangent * sin(phi);
      float azimuth = atan(position.z, position.x);
      float edge = smoothstep(6., 6.25, radius) * (1. - smoothstep(17.5, 18., radius));
      radiance += (1. - coverage) * diskLight(radius, azimuth, p.x * sin(inclination));
      coverage += (1. - coverage) * edge;
    }
  }
  // Modest optical halo. The photon/lensing arcs themselves come from traced rays.
  float glow = exp(-abs(impact - 5.25) * 2.3) * .075;
  glow *= smoothstep(4.95, 5.2, impact);
  radiance += vec3(1., .47, .15) * glow;
  vec3 color = 1. - exp(-radiance * 1.4);
  color = pow(color, vec3(.82));
  float alpha = clamp(max(max(color.r, color.g), color.b) * 1.8, 0., 1.);
  if (impact < 5.19615) alpha = 1.;
  // The shadow is photon capture, not a painted glowing event-horizon surface.
  gl_FragColor = vec4(color, alpha);
}`;

export function BlackHoleScene({ className = '' }: { className?: string }) {
  const host = useRef<HTMLDivElement>(null);
  const { paused } = useMotion();
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    const element = host.current;
    if (!element || paused || failed || !window.IntersectionObserver || !window.ResizeObserver) return;
    let disposed = false;
    let visible = false;
    let cleanupScene: (() => void) | undefined;
    let updatePlayback: (() => void) | undefined;
    let loading = false;

    async function initialize() {
      if (loading || disposed) return;
      loading = true;
      try {
        const [THREE, { animate }, geodesics] = await Promise.all([import('three'), import('animejs'), import('./schwarzschild')]);
        if (disposed) return;
        const renderer = new THREE.WebGLRenderer({ alpha: true, antialias: false, powerPreference: 'low-power' });
        renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 1.5));
        const lookup = new THREE.DataTexture(geodesics.getRayTable(), geodesics.RAY_TABLE_WIDTH, geodesics.RAY_TABLE_HEIGHT, THREE.RedFormat, THREE.FloatType);
        lookup.minFilter = THREE.NearestFilter;
        lookup.magFilter = THREE.NearestFilter;
        lookup.generateMipmaps = false;
        lookup.needsUpdate = true;
        const scene = new THREE.Scene();
        const camera = new THREE.OrthographicCamera(-1, 1, 1, -1, 0, 2);
        camera.position.z = 1;
        const geometry = new THREE.PlaneGeometry(2, 2);
        const material = new THREE.ShaderMaterial({
          uniforms: { phase: { value: 0 }, aspect: { value: 1 }, rayTable: { value: lookup } },
          vertexShader: 'varying vec2 vUv; void main() { vUv = uv; gl_Position = vec4(position.xy, 0., 1.); }',
          fragmentShader, transparent: true, depthTest: false, depthWrite: false,
        });
        scene.add(new THREE.Mesh(geometry, material));
        const canvas = renderer.domElement;
        canvas.setAttribute('aria-hidden', 'true');
        element!.appendChild(canvas);
        const state = { phase: 0 };
        const render = () => {
          material.uniforms.phase.value = state.phase;
          renderer.render(scene, camera);
        };
        // Anime.js owns the only frame loop. No React updates on frames.
        const animation = animate(state, {
          phase: Math.PI * 2, duration: 120000, ease: 'linear', loop: true,
          frameRate: 30, autoplay: false, onUpdate: render,
        });
        updatePlayback = () => {
          const active = visible && !document.hidden;
          if (active) animation.resume(); else animation.pause();
          element!.dataset.playback = active ? 'playing' : 'paused';
        };
        const resize = () => {
          const { width, height } = element!.getBoundingClientRect();
          if (!width || !height) return;
          // Bound the drawing buffer even on a very wide/retina display.
          const scale = Math.min(1, 900 / width, 700 / height);
          renderer.setSize(Math.round(width * scale), Math.round(height * scale), false);
          material.uniforms.aspect.value = width / height;
          if (visible && !document.hidden) render();
        };
        const resizeObserver = new ResizeObserver(resize);
        resizeObserver.observe(element!);
        const lost = (event: Event) => { event.preventDefault(); animation.pause(); setFailed(true); };
        canvas.addEventListener('webglcontextlost', lost);
        cleanupScene = () => {
          animation.revert();
          resizeObserver.disconnect();
          canvas.removeEventListener('webglcontextlost', lost);
          lookup.dispose(); geometry.dispose(); material.dispose(); renderer.dispose(); renderer.forceContextLoss();
          canvas.remove();
          delete element!.dataset.renderer;
          delete element!.dataset.playback;
        };
        resize();
        element!.dataset.renderer = 'webgl';
        updatePlayback();
      } catch {
        cleanupScene?.();
        cleanupScene = undefined;
        if (!disposed) setFailed(true);
      }
    }

    const observer = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting && entry.boundingClientRect.width > 0;
      if (visible) void initialize();
      updatePlayback?.();
    }, { threshold: .05 });
    observer.observe(element);
    const visibility = () => updatePlayback?.();
    document.addEventListener('visibilitychange', visibility);
    return () => {
      disposed = true;
      observer.disconnect();
      document.removeEventListener('visibilitychange', visibility);
      cleanupScene?.();
    };
  }, [paused, failed]);

  return <div className={`cz-blackhole-scene ${className}`}>
    <div ref={host} className="cz-blackhole-art" aria-hidden="true">
      <div className="cz-blackhole-fallback"><EventHorizon size={540} /></div>
    </div>
    <div className="cz-scene-controls">{failed ? <span>Still view · WebGL unavailable</span> : <MotionToggle />}</div>
  </div>;
}
