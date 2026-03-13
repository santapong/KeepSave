export function getWidgetStyles(theme: 'light' | 'dark'): string {
  const colors = theme === 'dark'
    ? {
        bg: '#1f2937',
        surface: '#374151',
        surfaceHover: '#4b5563',
        primary: '#3b82f6',
        primaryHover: '#2563eb',
        danger: '#ef4444',
        dangerHover: '#dc2626',
        success: '#22c55e',
        text: '#f9fafb',
        textSecondary: '#9ca3af',
        border: '#4b5563',
        inputBg: '#1f2937',
        maskBg: '#4b5563',
      }
    : {
        bg: '#f8f9fa',
        surface: '#ffffff',
        surfaceHover: '#f3f4f6',
        primary: '#2563eb',
        primaryHover: '#1d4ed8',
        danger: '#dc2626',
        dangerHover: '#b91c1c',
        success: '#16a34a',
        text: '#1f2937',
        textSecondary: '#6b7280',
        border: '#e5e7eb',
        inputBg: '#ffffff',
        maskBg: '#f3f4f6',
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
      --ks-color-danger: ${colors.danger};
      --ks-color-danger-hover: ${colors.dangerHover};
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
      color: #ffffff;
    }

    .ks-btn-primary:hover {
      background: var(--ks-color-primary-hover);
    }

    .ks-btn-danger {
      color: var(--ks-color-danger);
      border-color: var(--ks-color-danger);
    }

    .ks-btn-danger:hover {
      background: var(--ks-color-danger);
      color: #ffffff;
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
      background: #fef2f2;
      border: 1px solid #fecaca;
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
  `;
}
