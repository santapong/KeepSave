import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { HelpPage } from './HelpPage';

const availability = vi.hoisted(() => ({ available: ['secret_history', 'encrypted_recovery'] as string[], error: false, loaded: true }));
vi.mock('../hooks/useCapabilities', () => ({
  useCapabilities: () => ({
    capabilities: availability.loaded ? { restricted_profile: true, available: availability.available } : null,
    error: availability.error,
    enabled: (feature: string) => availability.available.includes(feature),
  }),
}));
function LocationProbe() {
  const location = useLocation();
  return <output aria-label="Current documentation URL">{location.pathname}{location.hash}</output>;
}
function renderHelp(path = '/help') {
  return render(<MemoryRouter initialEntries={[path]}><HelpPage /><LocationProbe /></MemoryRouter>);
}

describe('HelpPage task guides', () => {
  beforeEach(() => {
    availability.available = ['secret_history', 'encrypted_recovery'];
    availability.error = false;
    availability.loaded = true;
  });

  it('opens a shareable section and uses the actual restore precondition', () => {
    renderHelp('/help#history');
    const article = screen.getByRole('article', { name: 'History, restore & rotation' });
    expect(within(article).getByRole('heading', { name: 'History, restore & rotation' })).toBeInTheDocument();
    expect(article).toHaveTextContent('expected_current_revision');
    expect(within(article).getByText(/optional for legacy update callers/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Link to this guide' })).toHaveAttribute('href', '/help#history');
  });

  it('filters guides by topic and navigates to the selected result', async () => {
    const user = userEvent.setup();
    renderHelp();
    await user.type(screen.getByRole('searchbox', { name: 'Search documentation' }), '409');
    const nav = screen.getByRole('navigation', { name: 'Documentation sections' });
    expect(within(nav).getByRole('button', { name: 'Troubleshoot a request' })).toBeInTheDocument();
    expect(within(nav).queryByRole('button', { name: 'Run KeepSave locally' })).not.toBeInTheDocument();
    await user.click(within(nav).getByRole('button', { name: 'Troubleshoot a request' }));
    expect(screen.getByRole('article', { name: 'Troubleshoot a request' })).toBeInTheDocument();
    expect(screen.getByRole('status', { name: 'Documentation search results' })).toHaveTextContent('2 matching guides');
    expect(screen.getByLabelText('Current documentation URL')).toHaveTextContent('/help#troubleshooting');
  });

  it('provides a recoverable no-match state without hiding the current article', async () => {
    const user = userEvent.setup();
    renderHelp();
    await user.type(screen.getByRole('searchbox', { name: 'Search documentation' }), 'unfindable-topic');
    expect(screen.getByRole('status', { name: 'Documentation search results' })).toHaveTextContent('0 matching guides');
    expect(screen.getByRole('article', { name: 'Your first secret' })).toBeInTheDocument();
    await user.click(within(screen.getByRole('navigation', { name: 'Documentation sections' })).getByRole('button', { name: 'Clear search' }));
    expect(screen.getByRole('searchbox')).toHaveValue('');
    expect(screen.getByRole('status', { name: 'Documentation search results' })).toHaveTextContent('13 task guides');
  });

  it('keeps the mobile section selector functional and updates the URL', async () => {
    const user = userEvent.setup();
    renderHelp();
    await user.selectOptions(screen.getByRole('combobox', { name: 'Documentation section' }), 'api');
    const article = screen.getByRole('article', { name: 'Make an API request' });
    expect(article).toHaveTextContent('environment=alpha');
    expect(article).toHaveTextContent('missing_keys');
    expect(screen.getByLabelText('Current documentation URL')).toHaveTextContent('/help#api');
  });

  it('does not claim MCP is configured when the protocol capability is unavailable', () => {
    renderHelp('/help#mcp-server');
    expect(screen.getByRole('heading', { name: 'MCP protocol unavailable on this installation' })).toBeInTheDocument();
    expect(screen.queryByText(/npm install.*mcp-server/)).not.toBeInTheDocument();
  });

  it('reports failed capability lookup instead of assuming availability', () => {
    availability.loaded = false;
    availability.error = true;
    renderHelp('/help#developer-guide');
    expect(screen.getByLabelText('Instance availability')).toHaveTextContent('Feature availability could not be checked');
    expect(screen.getByRole('article', { name: 'Run KeepSave locally' })).toHaveTextContent('/readyz');
  });

  it('falls back safely for an unknown section', () => {
    renderHelp('/help#not-a-guide');
    expect(screen.getByRole('article', { name: 'Your first secret' })).toBeInTheDocument();
  });

  it('finds a guide by its shareable section name', async () => {
    const user = userEvent.setup();
    renderHelp();
    await user.type(screen.getByRole('searchbox', { name: 'Search documentation' }), 'troubleshooting');
    const nav = screen.getByRole('navigation', { name: 'Documentation sections' });
    expect(within(nav).getByRole('button', { name: 'Troubleshoot a request' })).toBeInTheDocument();
    expect(screen.getByRole('status', { name: 'Documentation search results' })).toHaveTextContent('1 matching guide');
  });
});
