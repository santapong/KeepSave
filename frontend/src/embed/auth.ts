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

export function createAuthHandshake(
  widgetId: string,
  onAuth: (token?: string, apiKey?: string) => void
): { start: () => void; destroy: () => void } {
  let handler: MessageHandler | null = null;

  function start(): void {
    handler = (event: MessageEvent) => {
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
    window.parent.postMessage(request, '*');
  }

  function destroy(): void {
    if (handler) {
      window.removeEventListener('message', handler);
      handler = null;
    }
  }

  return { start, destroy };
}
