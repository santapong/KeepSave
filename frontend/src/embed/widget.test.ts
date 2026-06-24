import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { WidgetRenderer } from './widget';
import type { KeepSaveAPI, Secret } from './api';

/** Build a fake KeepSaveAPI with controllable secret list + spy methods. */
function makeApi(secrets: Secret[]): KeepSaveAPI {
  return {
    setToken: vi.fn(),
    setApiKey: vi.fn(),
    isAuthenticated: () => true,
    listSecrets: vi.fn().mockResolvedValue(secrets),
    createSecret: vi.fn(),
    updateSecret: vi.fn(),
    deleteSecret: vi.fn().mockResolvedValue(undefined),
    batchGetSecrets: vi.fn(),
  } as unknown as KeepSaveAPI;
}

function makeRoot(): ShadowRoot {
  const host = document.createElement('div');
  document.body.appendChild(host);
  return host.attachShadow({ mode: 'open' });
}

const sampleSecrets: Secret[] = [
  {
    id: 's1',
    project_id: 'p1',
    environment_id: 'e1',
    key: 'API_KEY',
    value: 'super-secret-value',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  },
];

describe('WidgetRenderer DOM construction (FE-F06)', () => {
  let root: ShadowRoot;

  beforeEach(() => {
    root = makeRoot();
    Object.defineProperty(document, 'visibilityState', {
      configurable: true,
      get: () => 'visible',
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders secret keys and values as text, never as parsed HTML', async () => {
    const xssSecrets: Secret[] = [
      {
        id: 'x1',
        project_id: 'p1',
        environment_id: 'e1',
        key: '<img src=x onerror=alert(1)>',
        value: '<script>alert(2)</script>',
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
    ];
    const api = makeApi(xssSecrets);
    const renderer = new WidgetRenderer(root, api, 'p1', 'readwrite');
    renderer.initialRender();
    await renderer.loadSecrets();

    // The malicious key must appear verbatim as text content...
    const keyEl = root.querySelector('.ks-secret-key');
    expect(keyEl?.textContent).toBe('<img src=x onerror=alert(1)>');
    // ...and must NOT have created a real <img> element in the shadow tree.
    expect(root.querySelector('img')).toBeNull();
    expect(root.querySelector('script')).toBeNull();

    renderer.destroy();
  });

  it('masks values by default and reveals on toggle', async () => {
    const api = makeApi(sampleSecrets);
    const renderer = new WidgetRenderer(root, api, 'p1', 'read');
    renderer.initialRender();
    await renderer.loadSecrets();

    // Masked initially.
    expect(root.textContent).not.toContain('super-secret-value');
    const revealBtn = root.querySelector<HTMLButtonElement>('[data-action="toggle-reveal"]');
    expect(revealBtn).not.toBeNull();
    revealBtn!.click();

    expect(root.querySelector('.ks-secret-value')?.textContent).toBe('super-secret-value');
    renderer.destroy();
  });

  // FE-F04: hidden tab must immediately re-mask revealed secrets.
  it('re-masks revealed secrets when the document becomes hidden', async () => {
    const api = makeApi(sampleSecrets);
    const renderer = new WidgetRenderer(root, api, 'p1', 'read');
    renderer.initialRender();
    await renderer.loadSecrets();

    root.querySelector<HTMLButtonElement>('[data-action="toggle-reveal"]')!.click();
    expect(root.textContent).toContain('super-secret-value');

    Object.defineProperty(document, 'visibilityState', {
      configurable: true,
      get: () => 'hidden',
    });
    document.dispatchEvent(new Event('visibilitychange'));

    expect(root.textContent).not.toContain('super-secret-value');
    renderer.destroy();
  });

  // FE-F01/02/03: delete uses a typed-confirmation modal, not window.confirm.
  it('requires typing the key before delete fires (no window.confirm)', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm');
    const api = makeApi(sampleSecrets);
    const renderer = new WidgetRenderer(root, api, 'p1', 'readwrite');
    renderer.initialRender();
    await renderer.loadSecrets();

    root.querySelector<HTMLButtonElement>('[data-action="delete"]')!.click();

    // window.confirm must NOT be used.
    expect(confirmSpy).not.toHaveBeenCalled();

    // A typed-confirm modal is shown with a disabled confirm button.
    const overlay = root.querySelector('.ks-modal-overlay');
    expect(overlay).not.toBeNull();
    const buttons = Array.from(root.querySelectorAll<HTMLButtonElement>('.ks-modal button'));
    const confirmBtn = buttons.find((b) => b.textContent === 'Delete secret')!;
    expect(confirmBtn.disabled).toBe(true);
    expect(api.deleteSecret).not.toHaveBeenCalled();

    // Type the wrong value: stays disabled.
    const input = root.querySelector<HTMLInputElement>('[data-input="confirm-delete"]')!;
    input.value = 'WRONG';
    input.dispatchEvent(new Event('input'));
    expect(confirmBtn.disabled).toBe(true);

    // Type the exact key: enables and deletes.
    input.value = 'API_KEY';
    input.dispatchEvent(new Event('input'));
    expect(confirmBtn.disabled).toBe(false);
    confirmBtn.click();
    expect(api.deleteSecret).toHaveBeenCalledWith('p1', 's1');

    renderer.destroy();
  });

  it('removes the visibilitychange listener on destroy', async () => {
    const removeSpy = vi.spyOn(document, 'removeEventListener');
    const api = makeApi(sampleSecrets);
    const renderer = new WidgetRenderer(root, api, 'p1', 'read');
    renderer.initialRender();
    await renderer.loadSecrets();
    renderer.destroy();
    expect(removeSpy).toHaveBeenCalledWith('visibilitychange', expect.any(Function));
  });
});
