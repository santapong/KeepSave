import { render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { CapabilityProvider, CapabilityGate, useCapabilities } from './useCapabilities';
import { getCapabilities } from '../api/client';
vi.mock('../api/client', () => ({ getCapabilities: vi.fn() }));
function VaultAvailability() { const { enabled } = useCapabilities(); return <span>{enabled('secret_history') ? 'Vault enabled' : 'Vault unavailable'}</span>; }
it('blocks unavailable integration surfaces and requires actual instance vault support', async () => {
 vi.mocked(getCapabilities).mockResolvedValue({ profile: 'core', restricted_profile: true, application_origin: 'https://app.keepsave.draveniq.dev', landing_origin: 'https://keepsave.draveniq.dev', available: ['projects'], postgresql_only: ['secret_history'], unavailable: ['mcp_execution'] });
 render(<CapabilityProvider><VaultAvailability /><CapabilityGate feature="mcp_execution"><button>Execute connector</button></CapabilityGate></CapabilityProvider>);
 expect(await screen.findByText('Planned integration')).toBeInTheDocument(); expect(screen.queryByRole('button', { name: 'Execute connector' })).not.toBeInTheDocument(); expect(screen.getByText('Vault unavailable')).toBeInTheDocument();
});
