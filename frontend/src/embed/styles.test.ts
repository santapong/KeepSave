import { describe, it, expect } from 'vitest';
import { getWidgetStyles } from './styles';

describe('getWidgetStyles', () => {
  it('returns light theme styles by default', () => {
    const styles = getWidgetStyles('light');

    expect(styles).toContain(':host');
    expect(styles).toContain('--ks-color-bg: #f7f4ee');
    expect(styles).toContain('--ks-color-surface: #fffdf9');
    expect(styles).toContain('--ks-color-primary: #8b511d');
    expect(styles).toContain('--ks-color-text: #29251f');
  });

  it('returns dark theme styles', () => {
    const styles = getWidgetStyles('dark');

    expect(styles).toContain(':host');
    expect(styles).toContain('--ks-color-bg: #08090d');
    expect(styles).toContain('--ks-color-surface: #111217');
    expect(styles).toContain('--ks-color-primary: #f4bf76');
    expect(styles).toContain('--ks-color-text: #f5f1e8');
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
