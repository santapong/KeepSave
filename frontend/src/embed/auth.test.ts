import { describe, it, expect, vi, afterEach } from 'vitest';
import { createAuthHandshake } from './auth';

describe('createAuthHandshake', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('registers a message listener on start', () => {
    const addSpy = vi.spyOn(window, 'addEventListener');
    const onAuth = vi.fn();
    vi.spyOn(window.parent, 'postMessage').mockImplementation(() => {});

    const handshake = createAuthHandshake('widget-1', onAuth);
    handshake.start();

    expect(addSpy).toHaveBeenCalledWith('message', expect.any(Function));
    handshake.destroy();
  });

  it('sends auth request to parent on start', () => {
    const onAuth = vi.fn();
    const postMessageSpy = vi.spyOn(window.parent, 'postMessage').mockImplementation(() => {});

    const handshake = createAuthHandshake('test-id', onAuth);
    handshake.start();

    expect(postMessageSpy).toHaveBeenCalledWith(
      { type: 'keepsave-auth-request', widgetId: 'test-id' },
      '*'
    );

    handshake.destroy();
  });

  it('calls onAuth when receiving keepsave-auth message with token', () => {
    const onAuth = vi.fn();
    vi.spyOn(window.parent, 'postMessage').mockImplementation(() => {});

    const handshake = createAuthHandshake('w1', onAuth);
    handshake.start();

    const event = new MessageEvent('message', {
      data: { type: 'keepsave-auth', token: 'jwt-token-123' },
    });
    window.dispatchEvent(event);

    expect(onAuth).toHaveBeenCalledWith('jwt-token-123', undefined);

    handshake.destroy();
  });

  it('calls onAuth when receiving keepsave-auth message with apiKey', () => {
    const onAuth = vi.fn();
    vi.spyOn(window.parent, 'postMessage').mockImplementation(() => {});

    const handshake = createAuthHandshake('w1', onAuth);
    handshake.start();

    const event = new MessageEvent('message', {
      data: { type: 'keepsave-auth', apiKey: 'ks_test_key' },
    });
    window.dispatchEvent(event);

    expect(onAuth).toHaveBeenCalledWith(undefined, 'ks_test_key');

    handshake.destroy();
  });

  it('ignores unrelated messages', () => {
    const onAuth = vi.fn();
    vi.spyOn(window.parent, 'postMessage').mockImplementation(() => {});

    const handshake = createAuthHandshake('w1', onAuth);
    handshake.start();

    window.dispatchEvent(new MessageEvent('message', {
      data: { type: 'some-other-message' },
    }));

    window.dispatchEvent(new MessageEvent('message', {
      data: 'just a string',
    }));

    window.dispatchEvent(new MessageEvent('message', {
      data: null,
    }));

    expect(onAuth).not.toHaveBeenCalled();

    handshake.destroy();
  });

  it('removes listener on destroy', () => {
    const removeSpy = vi.spyOn(window, 'removeEventListener');
    const onAuth = vi.fn();
    vi.spyOn(window.parent, 'postMessage').mockImplementation(() => {});

    const handshake = createAuthHandshake('w1', onAuth);
    handshake.start();
    handshake.destroy();

    expect(removeSpy).toHaveBeenCalledWith('message', expect.any(Function));
  });
});
