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
  /** FE-F01/02/03: secret pending typed-confirmation delete, or null. */
  pendingDelete: { id: string; key: string } | null;
}

// FE-F06: All DOM in this widget is built with createElement / textContent /
// setAttribute. Assigning `innerHTML` from a template string is BANNED here —
// secret values and keys are attacker-influenced and an `innerHTML` sink is an
// XSS trap. An ESLint `no-restricted-properties` rule enforces this (see
// eslint.config.js); do not reintroduce string-HTML rendering.

type ElProps = {
  className?: string;
  text?: string;
  type?: string;
  placeholder?: string;
  value?: string;
  /** data-* attributes (set via setAttribute, value-escaped by the DOM). */
  dataset?: Record<string, string>;
};

/**
 * Safe element factory. Text content is always set via `textContent` and
 * attributes via `setAttribute`, so no markup in `text`/`value`/dataset values
 * is ever interpreted as HTML.
 */
function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  props: ElProps = {},
  children: (Node | null | undefined)[] = []
): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  if (props.className) node.className = props.className;
  if (props.text !== undefined) node.textContent = props.text;
  if (props.type !== undefined) node.setAttribute('type', props.type);
  if (props.placeholder !== undefined) node.setAttribute('placeholder', props.placeholder);
  if (props.value !== undefined) (node as HTMLInputElement).value = props.value;
  if (props.dataset) {
    for (const [k, v] of Object.entries(props.dataset)) {
      node.setAttribute(`data-${k}`, v);
    }
  }
  for (const child of children) {
    if (child) node.appendChild(child);
  }
  return node;
}

export class WidgetRenderer {
  private root: ShadowRoot;
  private api: KeepSaveAPI;
  private projectId: string;
  private mode: WidgetMode;
  private state: WidgetState;
  /** FE-F04: bound visibilitychange handler so it can be removed on destroy. */
  private onVisibilityChange: () => void;
  private visibilityBound = false;

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
      pendingDelete: null,
    };
    // FE-F04 / EMBED_STATE.md §"Auto-clear / timeout policy": when the tab is
    // hidden, immediately re-mask any revealed secret so plaintext never
    // persists in the DOM across a tab switch.
    this.onVisibilityChange = () => {
      if (document.visibilityState === 'hidden' && this.state.revealed.size > 0) {
        this.state.revealed = new Set();
        this.render();
      }
    };
  }

  /** Registers the visibilitychange listener once. */
  private ensureVisibilityListener(): void {
    if (this.visibilityBound) return;
    document.addEventListener('visibilitychange', this.onVisibilityChange);
    this.visibilityBound = true;
  }

  /** Removes global listeners. Call from the element's disconnectedCallback. */
  destroy(): void {
    if (this.visibilityBound) {
      document.removeEventListener('visibilitychange', this.onVisibilityChange);
      this.visibilityBound = false;
    }
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

    body.replaceChildren(this.renderBody());
    this.attachBodyListeners(body);
    this.syncConfirmModal();
  }

  renderFull(): void {
    const container = this.root.querySelector('.ks-widget-root');
    if (!container) return;

    container.replaceChildren(this.renderContainer());
    this.attachListeners(container);
    this.syncConfirmModal();
  }

  initialRender(): void {
    this.ensureVisibilityListener();
    const wrapper = this.ensureWrapper();
    wrapper.replaceChildren(this.renderContainer());
    this.attachListeners(wrapper);
    this.syncConfirmModal();
  }

  renderAuthPrompt(): void {
    const wrapper = this.ensureWrapper();
    wrapper.replaceChildren(this.renderAuthPromptNode());
  }

  /** Returns the existing root wrapper or creates and attaches a fresh one. */
  private ensureWrapper(): Element {
    let wrapper = this.root.querySelector('.ks-widget-root');
    if (!wrapper) {
      wrapper = document.createElement('div');
      wrapper.className = 'ks-widget-root';
      this.root.appendChild(wrapper);
    }
    return wrapper;
  }

  private renderAuthPromptNode(): HTMLElement {
    return el('div', { className: 'ks-container' }, [
      el('div', { className: 'ks-header' }, [
        el('span', { className: 'ks-header-title', text: 'KeepSave' }),
        el('span', { className: 'ks-status' }, [
          el('span', { className: 'ks-status-dot disconnected' }),
          document.createTextNode('Not connected'),
        ]),
      ]),
      el('div', { className: 'ks-auth-prompt' }, [
        el('p', { text: 'Waiting for authentication...' }),
        el('p', { text: 'The host page must provide credentials via postMessage.' }),
      ]),
    ]);
  }

  private renderContainer(): HTMLElement {
    const envs = ['alpha', 'uat', 'prod'];
    const tabs = envs.map((env) =>
      el('button', {
        className: env === this.state.environment ? 'ks-tab active' : 'ks-tab',
        text: env.toUpperCase(),
        dataset: { env },
      })
    );

    return el('div', { className: 'ks-container' }, [
      el('div', { className: 'ks-header' }, [
        el('span', { className: 'ks-header-title', text: 'KeepSave' }),
        el('span', { className: 'ks-status' }, [
          el('span', { className: 'ks-status-dot' }),
          document.createTextNode('Connected'),
        ]),
      ]),
      el('div', { className: 'ks-tabs' }, tabs),
      el('div', { className: 'ks-body' }, [this.renderBody()]),
    ]);
  }

  private renderBody(): DocumentFragment {
    const frag = document.createDocumentFragment();

    if (this.state.error) {
      frag.appendChild(el('div', { className: 'ks-error', text: this.state.error }));
    }

    if (this.state.loading) {
      frag.appendChild(el('div', { className: 'ks-loading', text: 'Loading secrets...' }));
      return frag;
    }

    if (this.mode === 'readwrite') {
      frag.appendChild(this.renderAddForm());
    }

    if (this.state.secrets.length === 0) {
      frag.appendChild(el('div', { className: 'ks-empty', text: 'No secrets in this environment.' }));
      return frag;
    }

    const list = el('ul', { className: 'ks-secret-list' });
    for (const secret of this.state.secrets) {
      list.appendChild(this.renderSecretItem(secret));
    }
    frag.appendChild(list);

    return frag;
  }

  private renderAddForm(): HTMLElement {
    if (!this.state.showAddForm) {
      const wrap = el('div', { className: 'ks-add-toggle' }, [
        el('button', {
          className: 'ks-btn ks-btn-primary',
          text: '+ Add Secret',
          dataset: { action: 'show-add' },
        }),
      ]);
      return wrap;
    }

    return el('div', { className: 'ks-add-form' }, [
      el('input', {
        className: 'ks-input',
        placeholder: 'KEY',
        value: this.state.newKey,
        dataset: { input: 'new-key' },
      }),
      el('input', {
        className: 'ks-input',
        placeholder: 'Value',
        type: 'password',
        value: this.state.newValue,
        dataset: { input: 'new-value' },
      }),
      el('button', {
        className: 'ks-btn ks-btn-primary ks-btn-sm',
        text: 'Add',
        dataset: { action: 'add' },
      }),
      el('button', {
        className: 'ks-btn ks-btn-sm',
        text: 'Cancel',
        dataset: { action: 'cancel-add' },
      }),
    ]);
  }

  private renderSecretItem(secret: Secret): HTMLElement {
    const isRevealed = this.state.revealed.has(secret.id);
    const isEditing = this.state.editingId === secret.id;

    let valueNode: HTMLElement;
    if (isEditing) {
      valueNode = el('div', { className: 'ks-edit-row' }, [
        el('input', {
          className: 'ks-input',
          value: this.state.editingValue,
          dataset: { input: 'edit-value' },
        }),
        el('button', {
          className: 'ks-btn ks-btn-primary ks-btn-sm',
          text: 'Save',
          dataset: { action: 'save-edit', id: secret.id },
        }),
        el('button', {
          className: 'ks-btn ks-btn-sm',
          text: 'Cancel',
          dataset: { action: 'cancel-edit' },
        }),
      ]);
    } else if (isRevealed) {
      valueNode = el('span', { className: 'ks-secret-value', text: secret.value || '' });
    } else {
      valueNode = el('span', {
        className: 'ks-secret-value ks-secret-mask',
        text: '••••••••',
      });
    }

    const actions = el('span', { className: 'ks-secret-actions' }, [
      el('button', {
        className: 'ks-btn ks-btn-sm',
        text: isRevealed ? 'Hide' : 'Reveal',
        dataset: { action: 'toggle-reveal', id: secret.id },
      }),
    ]);

    if (this.mode === 'readwrite' && !isEditing) {
      actions.appendChild(
        el('button', {
          className: 'ks-btn ks-btn-sm',
          text: 'Edit',
          dataset: { action: 'edit', id: secret.id, value: secret.value || '' },
        })
      );
      actions.appendChild(
        el('button', {
          className: 'ks-btn ks-btn-sm ks-btn-danger',
          text: 'Delete',
          dataset: { action: 'delete', id: secret.id, key: secret.key },
        })
      );
    }

    return el('li', { className: 'ks-secret-item' }, [
      el('span', { className: 'ks-secret-key', text: secret.key }),
      valueNode,
      actions,
    ]);
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
            // FE-F01/02/03: typed-confirmation modal instead of window.confirm.
            if (id) {
              this.state.pendingDelete = { id, key: btn.dataset.key || '' };
              this.syncConfirmModal();
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

  /**
   * FE-F01/02/03: typed-confirmation modal for destructive delete. Mirrors the
   * dashboard's <TypedConfirmModal>: the user must type the secret key before
   * the Delete button enables. Built entirely with createElement so no
   * attacker-controlled key reaches an innerHTML sink.
   */
  private syncConfirmModal(): void {
    const existing = this.root.querySelector('.ks-modal-overlay');
    if (existing) existing.remove();
    if (!this.state.pendingDelete) return;

    const { id, key } = this.state.pendingDelete;

    const confirmBtn = el('button', {
      className: 'ks-btn ks-btn-danger ks-btn-modal',
      text: 'Delete secret',
    });
    (confirmBtn as HTMLButtonElement).disabled = true;

    const input = el('input', {
      className: 'ks-input',
      placeholder: key,
      dataset: { input: 'confirm-delete' },
    });
    input.setAttribute('autocomplete', 'off');
    input.setAttribute('spellcheck', 'false');

    input.addEventListener('input', () => {
      (confirmBtn as HTMLButtonElement).disabled = (input as HTMLInputElement).value !== key;
    });

    const close = () => {
      this.state.pendingDelete = null;
      this.syncConfirmModal();
    };

    confirmBtn.addEventListener('click', () => {
      if ((input as HTMLInputElement).value !== key) return;
      this.state.pendingDelete = null;
      this.syncConfirmModal();
      this.deleteSecret(id);
    });

    const cancelBtn = el('button', { className: 'ks-btn ks-btn-modal', text: 'Cancel' });
    cancelBtn.addEventListener('click', close);

    const hint = el('div', { className: 'ks-modal-hint' }, [
      document.createTextNode('Type '),
      el('code', { text: key }),
      document.createTextNode(' to confirm deletion.'),
    ]);

    const dialog = el('div', { className: 'ks-modal' }, [
      el('div', { className: 'ks-modal-title', text: 'Delete secret' }),
      el('div', {
        className: 'ks-modal-desc',
        text: `This permanently deletes "${key}". This action cannot be undone.`,
      }),
      hint,
      input,
      el('div', { className: 'ks-modal-actions' }, [cancelBtn, confirmBtn]),
    ]);

    const overlay = el('div', { className: 'ks-modal-overlay' }, [dialog]);
    overlay.addEventListener('click', (e) => {
      if (e.target === overlay) close();
    });

    this.root.appendChild(overlay);
    setTimeout(() => (input as HTMLInputElement).focus(), 0);
  }
}
