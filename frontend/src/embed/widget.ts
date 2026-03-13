import { KeepSaveAPI, type Secret } from './api';

export type WidgetMode = 'read' | 'readwrite';

export interface WidgetState {
  environment: string;
  secrets: Secret[];
  loading: boolean;
  error: string | null;
  revealed: Set<string>;
  editingId: string | null;
  editingValue: string;
  showAddForm: boolean;
  newKey: string;
  newValue: string;
}

export class WidgetRenderer {
  private root: ShadowRoot;
  private api: KeepSaveAPI;
  private projectId: string;
  private mode: WidgetMode;
  private state: WidgetState;

  constructor(root: ShadowRoot, api: KeepSaveAPI, projectId: string, mode: WidgetMode) {
    this.root = root;
    this.api = api;
    this.projectId = projectId;
    this.mode = mode;
    this.state = {
      environment: 'alpha',
      secrets: [],
      loading: false,
      error: null,
      revealed: new Set(),
      editingId: null,
      editingValue: '',
      showAddForm: false,
      newKey: '',
      newValue: '',
    };
  }

  async loadSecrets(): Promise<void> {
    this.state.loading = true;
    this.state.error = null;
    this.render();

    try {
      this.state.secrets = await this.api.listSecrets(this.projectId, this.state.environment);
    } catch (err) {
      this.state.error = err instanceof Error ? err.message : 'Failed to load secrets';
    } finally {
      this.state.loading = false;
      this.render();
    }
  }

  private async addSecret(): Promise<void> {
    if (!this.state.newKey.trim() || !this.state.newValue.trim()) return;

    try {
      await this.api.createSecret(
        this.projectId,
        this.state.newKey.trim(),
        this.state.newValue.trim(),
        this.state.environment
      );
      this.state.newKey = '';
      this.state.newValue = '';
      this.state.showAddForm = false;
      await this.loadSecrets();
    } catch (err) {
      this.state.error = err instanceof Error ? err.message : 'Failed to add secret';
      this.render();
    }
  }

  private async saveEdit(secretId: string): Promise<void> {
    try {
      await this.api.updateSecret(this.projectId, secretId, this.state.editingValue);
      this.state.editingId = null;
      this.state.editingValue = '';
      await this.loadSecrets();
    } catch (err) {
      this.state.error = err instanceof Error ? err.message : 'Failed to update secret';
      this.render();
    }
  }

  private async deleteSecret(secretId: string): Promise<void> {
    try {
      await this.api.deleteSecret(this.projectId, secretId);
      await this.loadSecrets();
    } catch (err) {
      this.state.error = err instanceof Error ? err.message : 'Failed to delete secret';
      this.render();
    }
  }

  render(): void {
    const container = this.root.querySelector('.ks-widget-root');
    if (!container) return;

    const body = container.querySelector('.ks-body');
    if (!body) return;

    body.innerHTML = this.renderBody();
    this.attachBodyListeners(body);
  }

  renderFull(): void {
    const container = this.root.querySelector('.ks-widget-root');
    if (!container) return;

    container.innerHTML = this.renderContainer();
    this.attachListeners(container);
  }

  initialRender(): void {
    const wrapper = this.root.querySelector('.ks-widget-root') || document.createElement('div');
    wrapper.className = 'ks-widget-root';
    wrapper.innerHTML = this.renderContainer();

    if (!this.root.querySelector('.ks-widget-root')) {
      this.root.appendChild(wrapper);
    }

    this.attachListeners(wrapper);
  }

  renderAuthPrompt(): void {
    const wrapper = this.root.querySelector('.ks-widget-root') || document.createElement('div');
    wrapper.className = 'ks-widget-root';
    wrapper.innerHTML = `
      <div class="ks-container">
        <div class="ks-header">
          <span class="ks-header-title">KeepSave</span>
          <span class="ks-status">
            <span class="ks-status-dot disconnected"></span>
            Not connected
          </span>
        </div>
        <div class="ks-auth-prompt">
          <p>Waiting for authentication...</p>
          <p>The host page must provide credentials via postMessage.</p>
        </div>
      </div>
    `;

    if (!this.root.querySelector('.ks-widget-root')) {
      this.root.appendChild(wrapper);
    }
  }

  private renderContainer(): string {
    const envs = ['alpha', 'uat', 'prod'];
    const tabs = envs.map((env) => {
      const active = env === this.state.environment ? ' active' : '';
      return `<button class="ks-tab${active}" data-env="${env}">${env.toUpperCase()}</button>`;
    }).join('');

    return `
      <div class="ks-container">
        <div class="ks-header">
          <span class="ks-header-title">KeepSave</span>
          <span class="ks-status">
            <span class="ks-status-dot"></span>
            Connected
          </span>
        </div>
        <div class="ks-tabs">${tabs}</div>
        <div class="ks-body">${this.renderBody()}</div>
      </div>
    `;
  }

  private renderBody(): string {
    let html = '';

    if (this.state.error) {
      html += `<div class="ks-error">${this.escapeHtml(this.state.error)}</div>`;
    }

    if (this.state.loading) {
      return html + `<div class="ks-loading">Loading secrets...</div>`;
    }

    if (this.mode === 'readwrite') {
      html += this.renderAddForm();
    }

    if (this.state.secrets.length === 0) {
      return html + `<div class="ks-empty">No secrets in this environment.</div>`;
    }

    html += `<ul class="ks-secret-list">`;
    for (const secret of this.state.secrets) {
      html += this.renderSecretItem(secret);
    }
    html += `</ul>`;

    return html;
  }

  private renderAddForm(): string {
    if (!this.state.showAddForm) {
      return `<div style="margin-bottom: 12px;">
        <button class="ks-btn ks-btn-primary" data-action="show-add">+ Add Secret</button>
      </div>`;
    }

    return `
      <div class="ks-add-form">
        <input class="ks-input" placeholder="KEY" data-input="new-key" value="${this.escapeAttr(this.state.newKey)}" />
        <input class="ks-input" placeholder="Value" data-input="new-value" type="password" value="${this.escapeAttr(this.state.newValue)}" />
        <button class="ks-btn ks-btn-primary ks-btn-sm" data-action="add">Add</button>
        <button class="ks-btn ks-btn-sm" data-action="cancel-add">Cancel</button>
      </div>
    `;
  }

  private renderSecretItem(secret: Secret): string {
    const isRevealed = this.state.revealed.has(secret.id);
    const isEditing = this.state.editingId === secret.id;

    let valueHtml: string;
    if (isEditing) {
      valueHtml = `
        <div class="ks-edit-row">
          <input class="ks-input" data-input="edit-value" value="${this.escapeAttr(this.state.editingValue)}" />
          <button class="ks-btn ks-btn-primary ks-btn-sm" data-action="save-edit" data-id="${secret.id}">Save</button>
          <button class="ks-btn ks-btn-sm" data-action="cancel-edit">Cancel</button>
        </div>
      `;
    } else if (isRevealed) {
      valueHtml = `<span class="ks-secret-value">${this.escapeHtml(secret.value || '')}</span>`;
    } else {
      valueHtml = `<span class="ks-secret-value ks-secret-mask">••••••••</span>`;
    }

    let actionsHtml = `
      <button class="ks-btn ks-btn-sm" data-action="toggle-reveal" data-id="${secret.id}">
        ${isRevealed ? 'Hide' : 'Reveal'}
      </button>
    `;

    if (this.mode === 'readwrite' && !isEditing) {
      actionsHtml += `
        <button class="ks-btn ks-btn-sm" data-action="edit" data-id="${secret.id}" data-value="${this.escapeAttr(secret.value || '')}">Edit</button>
        <button class="ks-btn ks-btn-sm ks-btn-danger" data-action="delete" data-id="${secret.id}" data-key="${this.escapeAttr(secret.key)}">Delete</button>
      `;
    }

    return `
      <li class="ks-secret-item">
        <span class="ks-secret-key">${this.escapeHtml(secret.key)}</span>
        ${valueHtml}
        <span class="ks-secret-actions">${actionsHtml}</span>
      </li>
    `;
  }

  private attachListeners(container: Element): void {
    container.querySelectorAll<HTMLElement>('.ks-tab').forEach((tab) => {
      tab.addEventListener('click', () => {
        const env = tab.dataset.env;
        if (env && env !== this.state.environment) {
          this.state.environment = env;
          this.state.revealed = new Set();
          this.state.editingId = null;
          this.state.showAddForm = false;
          this.renderFull();
          this.loadSecrets();
        }
      });
    });

    const body = container.querySelector('.ks-body');
    if (body) {
      this.attachBodyListeners(body);
    }
  }

  private attachBodyListeners(body: Element): void {
    body.querySelectorAll<HTMLElement>('[data-action]').forEach((btn) => {
      btn.addEventListener('click', (e) => {
        e.preventDefault();
        const action = btn.dataset.action;
        const id = btn.dataset.id;

        switch (action) {
          case 'show-add':
            this.state.showAddForm = true;
            this.render();
            break;
          case 'cancel-add':
            this.state.showAddForm = false;
            this.state.newKey = '';
            this.state.newValue = '';
            this.render();
            break;
          case 'add':
            this.addSecret();
            break;
          case 'toggle-reveal':
            if (id) {
              if (this.state.revealed.has(id)) {
                this.state.revealed.delete(id);
              } else {
                this.state.revealed.add(id);
              }
              this.render();
            }
            break;
          case 'edit':
            if (id) {
              this.state.editingId = id;
              this.state.editingValue = btn.dataset.value || '';
              this.render();
            }
            break;
          case 'save-edit':
            if (id) {
              this.saveEdit(id);
            }
            break;
          case 'cancel-edit':
            this.state.editingId = null;
            this.state.editingValue = '';
            this.render();
            break;
          case 'delete':
            if (id && confirm(`Delete secret "${btn.dataset.key}"?`)) {
              this.deleteSecret(id);
            }
            break;
        }
      });
    });

    body.querySelectorAll<HTMLInputElement>('[data-input]').forEach((input) => {
      input.addEventListener('input', () => {
        const name = input.dataset.input;
        switch (name) {
          case 'new-key':
            this.state.newKey = input.value;
            break;
          case 'new-value':
            this.state.newValue = input.value;
            break;
          case 'edit-value':
            this.state.editingValue = input.value;
            break;
        }
      });
    });
  }

  private escapeHtml(str: string): string {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  }

  private escapeAttr(str: string): string {
    return str
      .replace(/&/g, '&amp;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }
}
