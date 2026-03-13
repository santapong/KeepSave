import { describe, it, expect } from 'vitest';
import { getWidgetStyles } from './styles';

describe('getWidgetStyles', () => {
  it('returns light theme styles by default', () => {
    const styles = getWidgetStyles('light');

    expect(styles).toContain(':host');
    expect(styles).toContain('--ks-color-bg: #f8f9fa');
    expect(styles).toContain('--ks-color-surface: #ffffff');
    expect(styles).toContain('--ks-color-primary: #2563eb');
    expect(styles).toContain('--ks-color-text: #1f2937');
  });

  it('returns dark theme styles', () => {
    const styles = getWidgetStyles('dark');

    expect(styles).toContain(':host');
    expect(styles).toContain('--ks-color-bg: #1f2937');
    expect(styles).toContain('--ks-color-surface: #374151');
    expect(styles).toContain('--ks-color-primary: #3b82f6');
    expect(styles).toContain('--ks-color-text: #f9fafb');
  });

  it('includes all required CSS classes', () => {
    const styles = getWidgetStyles('light');

    const requiredClasses = [
      '.ks-container',
      '.ks-header',
      '.ks-tabs',
      '.ks-tab',
      '.ks-body',
      '.ks-secret-list',
      '.ks-secret-item',
      '.ks-secret-key',
      '.ks-secret-value',
      '.ks-btn',
      '.ks-btn-primary',
      '.ks-btn-danger',
      '.ks-input',
      '.ks-error',
      '.ks-loading',
      '.ks-empty',
      '.ks-auth-prompt',
    ];

    for (const cls of requiredClasses) {
      expect(styles).toContain(cls);
    }
  });

  it('includes box-sizing reset', () => {
    const styles = getWidgetStyles('light');
    expect(styles).toContain('box-sizing: border-box');
  });
});
