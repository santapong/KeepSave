import { KeepSaveAPI } from './api';
import { createAuthHandshake } from './auth';
import { getWidgetStyles } from './styles';
import { WidgetRenderer, type WidgetMode } from './widget';

// ADR-0006: the widget must fetch its per-project allow-list from the server
// before accepting any postMessage. This shape mirrors the public
// /api/v1/embed-config/:project_id payload.
interface EmbedConfigResponse {
  project_id: string;
  allowed_origins: string[];
  embed_policy_enabled: boolean;
}

// Determine the origin of the page that embedded the widget. We prefer
// document.referrer (the framing parent's URL) and fall back to the widget's
// own origin for the "no iframe" case (e.g. direct-attribute mode). Returns
// null if neither is available or parseable.
function detectParentOrigin(): string | null {
  try {
    if (document.referrer) {
      return new URL(document.referrer).origin;
    }
  } catch {
    // fallthrough
  }
  if (window.location && window.location.origin) {
    return window.location.origin;
  }
  return null;
}

export class KeepSaveWidget extends HTMLElement {
  static get observedAttributes(): string[] {
    return ['project-id', 'api-url', 'theme', 'mode', 'api-key', 'token'];
  }

  private api: KeepSaveAPI | null = null;
  private renderer: WidgetRenderer | null = null;
  private authHandshake: { start: () => void; destroy: () => void } | null = null;
  private initialized = false;

  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
  }

  connectedCallback(): void {
    if (this.initialized) return;
    this.initialized = true;
    // setup is async (boot-sequence fetch); fire-and-forget is acceptable
    // because every UI path inside renders synchronously after the fetch.
    void this.setup();
  }

  disconnectedCallback(): void {
    this.authHandshake?.destroy();
    this.initialized = false;
  }

  attributeChangedCallback(name: string, oldValue: string | null, newValue: string | null): void {
    if (!this.initialized || oldValue === newValue) return;

    if (name === 'theme') {
      this.applyTheme();
    } else {
      void this.setup();
    }
  }

  private get projectId(): string {
    return this.getAttribute('project-id') || '';
  }

  private get apiUrl(): string {
    return this.getAttribute('api-url') || window.location.origin;
  }

  private get theme(): 'light' | 'dark' {
    const val = this.getAttribute('theme');
    return val === 'dark' ? 'dark' : 'light';
  }

  private get mode(): WidgetMode {
    const val = this.getAttribute('mode');
    return val === 'readwrite' ? 'readwrite' : 'read';
  }

  private get directToken(): string | null {
    return this.getAttribute('token');
  }

  private get directApiKey(): string | null {
    return this.getAttribute('api-key');
  }

  private async setup(): Promise<void> {
    if (!this.shadowRoot) return;

    this.authHandshake?.destroy();

    this.applyTheme();

    this.api = new KeepSaveAPI(this.apiUrl);
    this.renderer = new WidgetRenderer(this.shadowRoot, this.api, this.projectId, this.mode);

    if (this.directToken) {
      this.api.setToken(this.directToken);
      this.startWidget();
      return;
    }
    if (this.directApiKey) {
      this.api.setApiKey(this.directApiKey);
      this.startWidget();
      return;
    }

    // postMessage auth path — must consult the server-side allow-list first.
    await this.bootPostMessageAuth();
  }

  /**
   * ADR-0006 boot sequence for postMessage-authenticated widgets:
   *   1. Fetch /api/v1/embed-config/:project_id.
   *   2. If response is 404 OR embed_policy_enabled is false, refuse to render.
   *   3. Determine the parent origin (document.referrer → window.location).
   *   4. If the parent origin is NOT in allowed_origins, refuse to render.
   *   5. Start the auth handshake against the matched allowed origin.
   *
   * Refusal is permanent for the current setup() cycle; the widget renders an
   * inert auth-prompt UI and surfaces a console error so operators can debug.
   */
  private async bootPostMessageAuth(): Promise<void> {
    if (!this.renderer) return;

    if (!this.projectId) {
      // eslint-disable-next-line no-console
      console.error('KeepSave: project-id attribute is required');
      this.renderer.renderAuthPrompt();
      return;
    }

    let config: EmbedConfigResponse | null = null;
    try {
      const resp = await fetch(
        `${this.apiUrl.replace(/\/+$/, '')}/api/v1/embed-config/${encodeURIComponent(this.projectId)}`,
        { credentials: 'omit' }
      );
      if (resp.status === 404) {
        // eslint-disable-next-line no-console
        console.error('KeepSave: embed not enabled for this project');
        this.renderer.renderAuthPrompt();
        return;
      }
      if (!resp.ok) {
        // eslint-disable-next-line no-console
        console.error('KeepSave: failed to load embed config (status %d)', resp.status);
        this.renderer.renderAuthPrompt();
        return;
      }
      config = (await resp.json()) as EmbedConfigResponse;
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error('KeepSave: failed to fetch embed config', err);
      this.renderer.renderAuthPrompt();
      return;
    }

    if (!config || !config.embed_policy_enabled) {
      // eslint-disable-next-line no-console
      console.error('KeepSave: embed not enabled for this project');
      this.renderer.renderAuthPrompt();
      return;
    }

    // Strip any wildcard sentinel that may have slipped past the server-side
    // validator. The server also strips it, but defence-in-depth is cheap.
    const allowedOrigins = (config.allowed_origins || []).filter((o) => o !== '*');
    if (allowedOrigins.length === 0) {
      // eslint-disable-next-line no-console
      console.error(
        'KeepSave: project has no valid allowed_origins configured; refusing to render'
      );
      this.renderer.renderAuthPrompt();
      return;
    }

    const parentOrigin = detectParentOrigin();
    if (!parentOrigin || !allowedOrigins.includes(parentOrigin)) {
      // eslint-disable-next-line no-console
      console.error(
        'KeepSave: this origin (%o) is not authorized to embed the widget',
        parentOrigin
      );
      this.renderer.renderAuthPrompt();
      return;
    }

    this.renderer.renderAuthPrompt();
    this.startPostMessageAuth(parentOrigin);
  }

  private applyTheme(): void {
    if (!this.shadowRoot) return;

    let styleEl = this.shadowRoot.querySelector('style');
    if (!styleEl) {
      styleEl = document.createElement('style');
      this.shadowRoot.prepend(styleEl);
    }
    styleEl.textContent = getWidgetStyles(this.theme);
  }

  private startPostMessageAuth(allowedOrigin: string): void {
    const widgetId = this.id || `keepsave-${Date.now()}`;

    this.authHandshake = createAuthHandshake(
      widgetId,
      (token, apiKey) => {
        if (!this.api) return;

        if (token) {
          this.api.setToken(token);
        } else if (apiKey) {
          this.api.setApiKey(apiKey);
        }

        this.startWidget();
      },
      { allowedOrigin }
    );

    this.authHandshake.start();
  }

  private startWidget(): void {
    if (!this.renderer || !this.projectId) return;

    this.renderer.initialRender();
    this.renderer.loadSecrets();
  }
}

export function register(): void {
  if (!customElements.get('keepsave-widget')) {
    customElements.define('keepsave-widget', KeepSaveWidget);
  }
}
