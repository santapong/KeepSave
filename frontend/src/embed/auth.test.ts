// ADR-0006 tests for the embed-widget auth handshake.
//
// Replaces the pre-allowlist behaviour where:
//   - the listener accepted `keepsave-auth` from any origin
//   - the outbound auth-request used `'*'` as the target origin
//
// Covers:
//   - inbound message from allowedOrigin is accepted
//   - inbound message from a non-allowed origin is rejected (no onAuth call)
//     and a warning is logged
//   - outbound postMessage targets the specific origin, never `'*'`
//   - handshake refuses to start when allowedOrigin is `'*'` or empty
import { describe, it, expect, vi, afterEach } from 'vitest';
import { createAuthHandshake } from './auth';

describe('createAuthHandshake (ADR-0006)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('registers a message listener on start', () => {
    const addSpy = vi.spyOn(window, 'addEventListener');
    const onAuth = vi.fn();
    vi.spyOn(window.parent, 'postMessage').mockImplementation(() => {});

    const handshake = createAuthHandshake('widget-1', onAuth, {
      allowedOrigin: 'https://host.example',
    });
    handshake.start();

    expect(addSpy).toHaveBeenCalledWith('message', expect.any(Function));
    handshake.destroy();
  });

  it('sends auth request to the specific allowedOrigin, never "*"', () => {
    const onAuth = vi.fn();
    const postMessageSpy = vi
      .spyOn(window.parent, 'postMessage')
      .mockImplementation(() => {});

    const handshake = createAuthHandshake('test-id', onAuth, {
      allowedOrigin: 'https://host.example',
    });
    handshake.start();

    expect(postMessageSpy).toHaveBeenCalledWith(
      { type: 'keepsave-auth-request', widgetId: 'test-id' },
      'https://host.example'
    );
    // Regression guard: ensure no call used '*'.
    for (const call of postMessageSpy.mock.calls) {
      expect(call[1]).not.toBe('*');
    }

    handshake.destroy();
  });

  it('accepts keepsave-auth from the allowed origin', () => {
    const onAuth = vi.fn();
    vi.spyOn(window.parent, 'postMessage').mockImplementation(() => {});

    const handshake = createAuthHandshake('w1', onAuth, {
      allowedOrigin: 'https://host.example',
    });
    handshake.start();

    const event = new MessageEvent('message', {
      data: { type: 'keepsave-auth', token: 'jwt-token-123' },
      origin: 'https://host.example',
    });
    window.dispatchEvent(event);

    expect(onAuth).toHaveBeenCalledWith('jwt-token-123', undefined);

    handshake.destroy();
  });

  it('accepts apiKey auth from the allowed origin', () => {
    const onAuth = vi.fn();
    vi.spyOn(window.parent, 'postMessage').mockImplementation(() => {});

    const handshake = createAuthHandshake('w1', onAuth, {
      allowedOrigin: 'https://host.example',
    });
    handshake.start();

    const event = new MessageEvent('message', {
      data: { type: 'keepsave-auth', apiKey: 'ks_test_key' },
      origin: 'https://host.example',
    });
    window.dispatchEvent(event);

    expect(onAuth).toHaveBeenCalledWith(undefined, 'ks_test_key');

    handshake.destroy();
  });

  it('rejects messages from a non-allowed origin and warns', () => {
    const onAuth = vi.fn();
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
    vi.spyOn(window.parent, 'postMessage').mockImplementation(() => {});

    const handshake = createAuthHandshake('w1', onAuth, {
      allowedOrigin: 'https://host.example',
    });
    handshake.start();

    const event = new MessageEvent('message', {
      data: { type: 'keepsave-auth', token: 'attacker-token' },
      origin: 'https://attacker.example',
    });
    window.dispatchEvent(event);

    expect(onAuth).not.toHaveBeenCalled();
    expect(warnSpy).toHaveBeenCalled();
    // Sanity-check the warning mentions both origins so an operator can
    // debug a misconfigured integrator.
    const firstWarn = warnSpy.mock.calls[0]?.join(' ') ?? '';
    expect(firstWarn).toMatch(/origin/i);

    handshake.destroy();
  });

  it('ignores unrelated messages from the allowed origin', () => {
    const onAuth = vi.fn();
    vi.spyOn(window.parent, 'postMessage').mockImplementation(() => {});

    const handshake = createAuthHandshake('w1', onAuth, {
      allowedOrigin: 'https://host.example',
    });
    handshake.start();

    window.dispatchEvent(
      new MessageEvent('message', {
        data: { type: 'some-other-message' },
        origin: 'https://host.example',
      })
    );
    window.dispatchEvent(
      new MessageEvent('message', {
        data: 'just a string',
        origin: 'https://host.example',
      })
    );
    window.dispatchEvent(
      new MessageEvent('message', {
        data: null,
        origin: 'https://host.example',
      })
    );

    expect(onAuth).not.toHaveBeenCalled();

    handshake.destroy();
  });

  it('refuses to start when allowedOrigin is "*" (defence-in-depth)', () => {
    const onAuth = vi.fn();
    const postMessageSpy = vi
      .spyOn(window.parent, 'postMessage')
      .mockImplementation(() => {});
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

    const handshake = createAuthHandshake('w1', onAuth, { allowedOrigin: '*' });
    handshake.start();

    // No listener registered, no outbound message, no auth.
    expect(postMessageSpy).not.toHaveBeenCalled();
    expect(errorSpy).toHaveBeenCalled();

    // Even if a "valid"-looking message arrives, onAuth must not fire.
    window.dispatchEvent(
      new MessageEvent('message', {
        data: { type: 'keepsave-auth', token: 't' },
        origin: 'https://anywhere.example',
      })
    );
    expect(onAuth).not.toHaveBeenCalled();

    handshake.destroy();
  });

  it('refuses to start when allowedOrigin is empty', () => {
    const onAuth = vi.fn();
    const postMessageSpy = vi
      .spyOn(window.parent, 'postMessage')
      .mockImplementation(() => {});
    vi.spyOn(console, 'error').mockImplementation(() => {});

    const handshake = createAuthHandshake('w1', onAuth, { allowedOrigin: '' });
    handshake.start();

    expect(postMessageSpy).not.toHaveBeenCalled();
    expect(onAuth).not.toHaveBeenCalled();

    handshake.destroy();
  });

  it('removes listener on destroy', () => {
    const removeSpy = vi.spyOn(window, 'removeEventListener');
    const onAuth = vi.fn();
    vi.spyOn(window.parent, 'postMessage').mockImplementation(() => {});

    const handshake = createAuthHandshake('w1', onAuth, {
      allowedOrigin: 'https://host.example',
    });
    handshake.start();
    handshake.destroy();

    expect(removeSpy).toHaveBeenCalledWith('message', expect.any(Function));
  });
});
