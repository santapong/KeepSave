export function getWidgetStyles(theme: 'light' | 'dark'): string {
  const colors = theme === 'dark'
    ? {
        bg: '#08090d',
        surface: '#111217',
        surfaceHover: '#241e1c',
        primary: '#f4bf76',
        primaryHover: '#f8cc91',
        primaryForeground: '#21170e',
        danger: '#f19d9d',
        dangerHover: '#f6b1b1',
        dangerForeground: '#2b1015',
        success: '#8bdbb1',
        text: '#f5f1e8',
        textSecondary: '#b5b2b8',
        border: '#353139',
        inputBg: '#111217',
        maskBg: '#19191f',
      }
    : {
        bg: '#f7f4ee',
        surface: '#fffdf9',
        surfaceHover: '#eee7dc',
        primary: '#8b511d',
        primaryHover: '#784316',
        primaryForeground: '#ffffff',
        danger: '#b52d45',
        dangerHover: '#a0253c',
        dangerForeground: '#ffffff',
        success: '#187344',
        text: '#29251f',
        textSecondary: '#706657',
        border: '#d9cdbd',
        inputBg: '#fffdf9',
        maskBg: '#eee7dc',
      };

  return `
    :host {
      display: block;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      font-size: 14px;
      line-height: 1.5;
      color: ${colors.text};

      --ks-color-bg: ${colors.bg};
      --ks-color-surface: ${colors.surface};
      --ks-color-surface-hover: ${colors.surfaceHover};
      --ks-color-primary: ${colors.primary};
      --ks-color-primary-hover: ${colors.primaryHover};
      --ks-color-primary-foreground: ${colors.primaryForeground};
      --ks-color-danger: ${colors.danger};
      --ks-color-danger-hover: ${colors.dangerHover};
      --ks-color-danger-foreground: ${colors.dangerForeground};
      --ks-color-success: ${colors.success};
      --ks-color-text: ${colors.text};
      --ks-color-text-secondary: ${colors.textSecondary};
      --ks-color-border: ${colors.border};
      --ks-color-input-bg: ${colors.inputBg};
      --ks-color-mask-bg: ${colors.maskBg};
    }

    * {
      box-sizing: border-box;
      margin: 0;
      padding: 0;
    }

    .ks-container {
      background: var(--ks-color-bg);
      border: 1px solid var(--ks-color-border);
      border-radius: 8px;
      overflow: hidden;
    }

    .ks-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 12px 16px;
      background: var(--ks-color-surface);
      border-bottom: 1px solid var(--ks-color-border);
    }

    .ks-header-title {
      font-size: 14px;
      font-weight: 600;
      color: var(--ks-color-text);
    }

    .ks-status {
      display: flex;
      align-items: center;
      gap: 6px;
      font-size: 12px;
      color: var(--ks-color-text-secondary);
    }

    .ks-status-dot {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      background: var(--ks-color-success);
    }

    .ks-status-dot.disconnected {
      background: var(--ks-color-danger);
    }

    .ks-tabs {
      display: flex;
      border-bottom: 1px solid var(--ks-color-border);
      background: var(--ks-color-surface);
    }

    .ks-tab {
      padding: 8px 16px;
      font-size: 13px;
      font-weight: 500;
      color: var(--ks-color-text-secondary);
      background: none;
      border: none;
      border-bottom: 2px solid transparent;
      cursor: pointer;
      transition: color 0.15s, border-color 0.15s;
    }

    .ks-tab:hover {
      color: var(--ks-color-text);
    }

    .ks-tab.active {
      color: var(--ks-color-primary);
      border-bottom-color: var(--ks-color-primary);
    }

    .ks-body {
      padding: 16px;
    }

    .ks-secret-list {
      list-style: none;
    }

    .ks-secret-item {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 10px 12px;
      background: var(--ks-color-surface);
      border: 1px solid var(--ks-color-border);
      border-radius: 6px;
      margin-bottom: 8px;
    }

    .ks-secret-key {
      font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
      font-size: 13px;
      font-weight: 600;
      color: var(--ks-color-text);
      min-width: 120px;
    }

    .ks-secret-value {
      font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
      font-size: 13px;
      color: var(--ks-color-text-secondary);
      flex: 1;
      margin: 0 12px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .ks-secret-mask {
      letter-spacing: 2px;
      color: var(--ks-color-text-secondary);
    }

    .ks-secret-actions {
      display: flex;
      gap: 6px;
      flex-shrink: 0;
    }

    .ks-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      padding: 6px 12px;
      font-size: 12px;
      font-weight: 500;
      border: 1px solid var(--ks-color-border);
      border-radius: 4px;
      background: var(--ks-color-surface);
      color: var(--ks-color-text);
      cursor: pointer;
      transition: background 0.15s, border-color 0.15s;
      white-space: nowrap;
    }

    .ks-btn:hover {
      background: var(--ks-color-surface-hover);
    }

    .ks-btn-primary {
      background: var(--ks-color-primary);
      border-color: var(--ks-color-primary);
      color: var(--ks-color-primary-foreground);
    }

    .ks-btn-primary:hover {
      background: var(--ks-color-primary-hover);
    }

    .ks-btn-danger {
      color: var(--ks-color-danger);
      border-color: var(--ks-color-danger);
    }

    .ks-btn-danger:hover {
      background: var(--ks-color-danger-hover);
      color: var(--ks-color-danger-foreground);
    }

    .ks-btn-sm {
      padding: 4px 8px;
      font-size: 11px;
    }

    .ks-add-form {
      display: flex;
      gap: 8px;
      margin-bottom: 12px;
    }

    .ks-input {
      flex: 1;
      padding: 6px 10px;
      font-size: 13px;
      border: 1px solid var(--ks-color-border);
      border-radius: 4px;
      background: var(--ks-color-input-bg);
      color: var(--ks-color-text);
      outline: none;
      transition: border-color 0.15s;
    }

    .ks-input:focus {
      border-color: var(--ks-color-primary);
    }

    .ks-input::placeholder {
      color: var(--ks-color-text-secondary);
    }

    .ks-error {
      padding: 10px 12px;
      background: color-mix(in srgb, var(--ks-color-danger) 9%, var(--ks-color-bg));
      border: 1px solid color-mix(in srgb, var(--ks-color-danger) 35%, var(--ks-color-bg));
      border-radius: 6px;
      color: var(--ks-color-danger);
      font-size: 13px;
      margin-bottom: 12px;
    }

    .ks-loading {
      text-align: center;
      padding: 24px;
      color: var(--ks-color-text-secondary);
    }

    .ks-empty {
      text-align: center;
      padding: 24px;
      color: var(--ks-color-text-secondary);
      font-size: 13px;
    }

    .ks-auth-prompt {
      text-align: center;
      padding: 32px 16px;
    }

    .ks-auth-prompt p {
      margin-bottom: 12px;
      color: var(--ks-color-text-secondary);
    }

    .ks-edit-row {
      display: flex;
      gap: 8px;
      align-items: center;
      flex: 1;
      margin: 0 12px;
    }

    .ks-add-toggle {
      margin-bottom: 12px;
    }

    /* FE-F01/02/03: typed-confirmation modal (replaces window.confirm). */
    .ks-modal-overlay {
      position: fixed;
      inset: 0;
      background: rgba(0, 0, 0, 0.5);
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: 2147483647;
    }

    .ks-modal {
      background: var(--ks-color-surface);
      border: 1px solid var(--ks-color-border);
      border-radius: 8px;
      padding: 20px;
      width: 90%;
      max-width: 420px;
      box-shadow: 0 10px 40px rgba(0, 0, 0, 0.3);
    }

    .ks-modal-title {
      font-size: 15px;
      font-weight: 600;
      color: var(--ks-color-text);
      margin-bottom: 8px;
    }

    .ks-modal-desc {
      font-size: 13px;
      color: var(--ks-color-text-secondary);
      margin-bottom: 12px;
    }

    .ks-modal-hint {
      font-size: 12px;
      color: var(--ks-color-text-secondary);
      margin-bottom: 6px;
    }

    .ks-modal-hint code {
      font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
      background: var(--ks-color-mask-bg);
      padding: 1px 4px;
      border-radius: 3px;
      color: var(--ks-color-text);
    }

    .ks-modal .ks-input {
      width: 100%;
      margin-bottom: 16px;
    }

    .ks-modal-actions {
      display: flex;
      justify-content: flex-end;
      gap: 8px;
    }

    .ks-btn-modal {
      padding: 8px 14px;
      font-size: 13px;
    }

    .ks-btn-danger:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    .ks-btn-danger:disabled:hover {
      background: var(--ks-color-surface);
      color: var(--ks-color-danger);
    }
  `;
}
