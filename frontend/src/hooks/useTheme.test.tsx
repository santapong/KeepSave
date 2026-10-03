import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useTheme } from './useTheme';

function ThemeControl({ label }: { label: string }) {
  const { theme, toggle } = useTheme();
  return <button onClick={toggle}>{label}: {theme}</button>;
}

describe('shared theme preference', () => {
  beforeEach(() => {
    localStorage.removeItem('keepsave_theme');
    document.documentElement.dataset.theme = 'dark';
  });
  afterEach(() => vi.restoreAllMocks());

  it('synchronizes desktop and mobile controls and persists the selection', async () => {
    const user = userEvent.setup();
    render(<><ThemeControl label="Desktop" /><ThemeControl label="Mobile" /></>);
    await user.click(screen.getByRole('button', { name: 'Mobile: dark' }));
    expect(screen.getByRole('button', { name: 'Desktop: light' })).toBeInTheDocument();
    expect(document.documentElement.dataset.theme).toBe('light');
    expect(localStorage.getItem('keepsave_theme')).toBe('light');
    await user.click(screen.getByRole('button', { name: 'Desktop: light' }));
    expect(screen.getByRole('button', { name: 'Mobile: dark' })).toBeInTheDocument();
  });

  it('still switches themes when browser storage is unavailable', async () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('Denied'); });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('Denied'); });
    const user = userEvent.setup();
    render(<ThemeControl label="Theme" />);
    await user.click(screen.getByRole('button', { name: 'Theme: dark' }));
    expect(screen.getByRole('button', { name: 'Theme: light' })).toBeInTheDocument();
  });
});
