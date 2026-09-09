import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { BlackHoleScene } from './BlackHoleScene';

const mocks = vi.hoisted(() => ({
  render: vi.fn(), dispose: vi.fn(), release: vi.fn(), materialDispose: vi.fn(), geometryDispose: vi.fn(), textureDispose: vi.fn(),
  pause: vi.fn(), resume: vi.fn(), revert: vi.fn(), animate: vi.fn(), fail: false,
}));
vi.mock('animejs', () => ({ animate: mocks.animate }));
vi.mock('three', () => ({
  WebGLRenderer: class {
    domElement = document.createElement('canvas');
    constructor() { if (mocks.fail) throw new Error('WebGL unavailable'); }
    setPixelRatio() {} setSize() {}
    render = mocks.render; dispose = mocks.dispose; forceContextLoss = mocks.release;
  },
  DataTexture: class { dispose = mocks.textureDispose; },
  RedFormat: 1028, FloatType: 1015, NearestFilter: 1003,
  Scene: class { add() {} },
  OrthographicCamera: class { position = { z: 0 }; },
  PlaneGeometry: class { dispose = mocks.geometryDispose; },
  ShaderMaterial: class { uniforms = { phase: { value: 0 }, aspect: { value: 1 } }; dispose = mocks.materialDispose; },
  Mesh: class {},
}));
let intersect: (entries: Partial<IntersectionObserverEntry>[]) => void;
let reduce = false;
const observerDisconnect = vi.fn();
beforeEach(() => {
  vi.clearAllMocks(); mocks.fail = false; reduce = false;
  localStorage.clear(); delete document.documentElement.dataset.motion;
  mocks.animate.mockReturnValue({ pause: mocks.pause, resume: mocks.resume, revert: mocks.revert });
  vi.stubGlobal('matchMedia', () => ({ matches: reduce, addEventListener() {}, removeEventListener() {} }));
  vi.stubGlobal('IntersectionObserver', class {
    constructor(callback: typeof intersect) { intersect = callback; }
    observe() {} disconnect = observerDisconnect;
  });
  vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} });
});
afterEach(() => { cleanup(); vi.unstubAllGlobals(); localStorage.clear(); delete document.documentElement.dataset.motion; });
async function show() {
  act(() => intersect([{ isIntersecting: true, boundingClientRect: { width: 600 } as DOMRectReadOnly }]));
  await waitFor(() => expect(document.querySelector('canvas')).not.toBeNull());
}

describe('Black hole resource lifecycle', () => {
  it('loads only when visible, pauses offscreen, and releases GPU resources on unmount', async () => {
    const { unmount } = render(<BlackHoleScene />);
    expect(mocks.animate).not.toHaveBeenCalled();
    await show();
    expect(mocks.animate).toHaveBeenCalledWith(expect.anything(), expect.objectContaining({ frameRate: 30, autoplay: false }));
    expect(mocks.resume).toHaveBeenCalled();
    act(() => intersect([{ isIntersecting: false, boundingClientRect: { width: 600 } as DOMRectReadOnly }]));
    expect(mocks.pause).toHaveBeenCalled();
    unmount();
    expect(mocks.revert).toHaveBeenCalledTimes(1);
    expect(mocks.dispose).toHaveBeenCalledTimes(1);
    expect(mocks.release).toHaveBeenCalledTimes(1);
    expect(mocks.geometryDispose).toHaveBeenCalledTimes(1);
    expect(mocks.materialDispose).toHaveBeenCalledTimes(1);
    expect(mocks.textureDispose).toHaveBeenCalledTimes(1);
    expect(observerDisconnect).toHaveBeenCalled();
  });
  it('never initializes WebGL for reduced motion', () => {
    reduce = true;
    render(<BlackHoleScene />);
    expect(screen.getByRole('button', { name: /system preference/i })).toBeDisabled();
    expect(mocks.animate).not.toHaveBeenCalled();
    expect(document.querySelector('canvas')).toBeNull();
  });
  it('persists explicit pause and removes the renderer', async () => {
    render(<BlackHoleScene />); await show();
    fireEvent.click(screen.getByRole('button', { name: 'Pause animation' }));
    expect(document.querySelector('canvas')).toBeNull();
    expect(localStorage.getItem('keepsave_motion')).toBe('off');
    expect(mocks.revert).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('button', { name: 'Resume animation' })).toHaveAttribute('aria-pressed', 'true');
  });
  it('keeps the fallback usable when WebGL creation fails', async () => {
    mocks.fail = true;
    render(<BlackHoleScene />);
    act(() => intersect([{ isIntersecting: true, boundingClientRect: { width: 600 } as DOMRectReadOnly }]));
    expect(await screen.findByText(/WebGL unavailable/)).toBeInTheDocument();
    expect(document.querySelector('canvas')).toBeNull();
  });
  it('stops and disposes a lost WebGL context', async () => {
    render(<BlackHoleScene />); await show();
    fireEvent(document.querySelector('canvas')!, new Event('webglcontextlost', { cancelable: true }));
    expect(await screen.findByText(/WebGL unavailable/)).toBeInTheDocument();
    expect(mocks.revert).toHaveBeenCalledTimes(1);
    expect(document.querySelector('canvas')).toBeNull();
  });
});
