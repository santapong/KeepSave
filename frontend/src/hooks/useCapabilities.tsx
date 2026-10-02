import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import { getCapabilities } from '../api/client';
import type { Capabilities } from '../api/coreTypes';

interface State { capabilities: Capabilities | null; error: boolean }
const CapabilityContext = createContext<State>({ capabilities: null, error: false });
export function CapabilityProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<State>({ capabilities: null, error: false });
  useEffect(() => {
    let active = true;
    getCapabilities().then((capabilities) => { if (active) setState({ capabilities, error: false }); })
      .catch(() => { if (active) setState({ capabilities: null, error: true }); });
    return () => { active = false; };
  }, []);
  return <CapabilityContext.Provider value={state}>{children}</CapabilityContext.Provider>;
}
export function useCapabilities() {
  const state = useContext(CapabilityContext);
  return { ...state, enabled: (feature: string) => !!state.capabilities?.available.includes(feature),
    unavailable: (feature: string) => !state.capabilities || (state.capabilities.restricted_profile && state.capabilities.unavailable.includes(feature)) };
}
export function routeCapability(path: string): string | null {
  if (path.startsWith('/mcp-hub')) return 'mcp_execution';
  if (path.startsWith('/oauth-clients')) return 'legacy_oauth';
  if (path.startsWith('/ai')) return 'experimental_intelligence';
  return null;
}
export function CapabilityGate({ feature, children }: { feature: string; children: ReactNode }) {
  const { unavailable, capabilities, error } = useCapabilities();
  if (!capabilities) return <div className="cz-page"><p role="status">{error ? 'Could not verify the available features. Reload to try again.' : 'Checking available features…'}</p></div>;
  if (unavailable(feature)) return <div className="cz-page"><h1>Planned integration</h1><p className="cz-muted">This feature is unavailable in this release. Your vault and account controls remain available.</p></div>;
  return children;
}
