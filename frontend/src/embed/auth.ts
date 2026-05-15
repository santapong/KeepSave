// ADR-0006: this module previously accepted `keepsave-auth` messages from
// any origin and used `*` as the outbound target. Both have been replaced
// with strict origin equality checks against the per-widget `allowedOrigin`
// resolved at boot from the server-side allow-list.
//
// See docs/EMBED_ORIGIN_POLICY.md §2 and docs/audits/SECURITY_AUDIT_2026-05-15.md
// finding A04-F2 for the original report and exploit repro.
export interface AuthMessage {
  type: 'keepsave-auth';
  token?: string;
  apiKey?: string;
}

export interface AuthRequestMessage {
  type: 'keepsave-auth-request';
  widgetId: string;
}

export type MessageHandler = (event: MessageEvent) => void;

export interface AuthHandshakeOptions {
  /**
   * The exact origin (scheme://host[:port]) the widget will accept messages
   * from AND post messages to. ADR-0006 forbids `'*'` here; if the caller
   * accidentally passes it, the handshake refuses to start and onAuth is
   * never invoked.
   */
  allowedOrigin: string;
}

export function createAuthHandshake(
  widgetId: string,
  onAuth: (token?: string, apiKey?: string) => void,
  options: AuthHandshakeOptions
): { start: () => void; destroy: () => void } {
  let handler: MessageHandler | null = null;
  const allowedOrigin = options.allowedOrigin;

  function start(): void {
    // Defence in depth: even if the boot path's allow-list check were
    // bypassed, refusing the wildcard sentinel here would catch a regression.
    if (!allowedOrigin || allowedOrigin === '*') {
      // eslint-disable-next-line no-console
      console.error(
        'KeepSave: refusing to start auth handshake without a specific allowedOrigin (got %o)',
        allowedOrigin
      );
      return;
    }

    handler = (event: MessageEvent) => {
      // Strict origin equality — drop silently per policy §2 to avoid
      // giving an attacker a probing oracle.
      if (event.origin !== allowedOrigin) {
        // eslint-disable-next-line no-console
        console.warn(
          'KeepSave: rejected postMessage from unexpected origin %o (expected %o)',
          event.origin,
          allowedOrigin
        );
        return;
      }
      const data = event.data;
      if (data && data.type === 'keepsave-auth' && (data.token || data.apiKey)) {
        onAuth(data.token, data.apiKey);
      }
    };
    window.addEventListener('message', handler);

    const request: AuthRequestMessage = {
      type: 'keepsave-auth-request',
      widgetId,
    };
    // Specific target origin — never `'*'`. If the parent's origin doesn't
    // match, the message is dropped silently by the browser (which is the
    // desired behaviour).
    window.parent.postMessage(request, allowedOrigin);
  }

  function destroy(): void {
    if (handler) {
      window.removeEventListener('message', handler);
      handler = null;
    }
  }

  return { start, destroy };
}
