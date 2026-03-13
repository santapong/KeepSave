import { KeepSaveAPI } from './api';
import { createAuthHandshake } from './auth';
import { getWidgetStyles } from './styles';
import { WidgetRenderer, type WidgetMode } from './widget';

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
    this.setup();
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
      this.setup();
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

  private setup(): void {
    if (!this.shadowRoot) return;

    this.authHandshake?.destroy();

    this.applyTheme();

    this.api = new KeepSaveAPI(this.apiUrl);
    this.renderer = new WidgetRenderer(this.shadowRoot, this.api, this.projectId, this.mode);

    if (this.directToken) {
      this.api.setToken(this.directToken);
      this.startWidget();
    } else if (this.directApiKey) {
      this.api.setApiKey(this.directApiKey);
      this.startWidget();
    } else {
      this.renderer.renderAuthPrompt();
      this.setupPostMessageAuth();
    }
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

  private setupPostMessageAuth(): void {
    const widgetId = this.id || `keepsave-${Date.now()}`;

    this.authHandshake = createAuthHandshake(widgetId, (token, apiKey) => {
      if (!this.api) return;

      if (token) {
        this.api.setToken(token);
      } else if (apiKey) {
        this.api.setApiKey(apiKey);
      }

      this.startWidget();
    });

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
