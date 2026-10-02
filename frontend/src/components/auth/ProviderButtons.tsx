import { useEffect, useState } from 'react';
import { LoaderCircle } from '@/components/icons';
import { getProviders, startSocialLogin, type Provider, type Providers } from '../../api/socialAuth';
export function ProviderButtons({ busy = false, onBusyChange }: { busy?: boolean; onBusyChange?: (busy: boolean) => void }) {
  const [providers, setProviders] = useState<Providers | null>(null);
  const [pending, setPending] = useState<Provider | null>(null);
  const [error, setError] = useState('');
  useEffect(() => {
    let active = true;
    getProviders().then((value) => { if (active) setProviders(value); }).catch(() => { if (active) setError('Provider sign-in is unavailable. You can still continue with email.'); });
    return () => { active = false; };
  }, []);
  async function begin(provider: Provider) {
    setError(''); setPending(provider); onBusyChange?.(true);
    try { await startSocialLogin(provider); }
    catch (error) { setError(error instanceof Error ? error.message : 'Could not start sign-in.'); setPending(null); onBusyChange?.(false); }
  }
  const unavailable = providers && (!providers.github || !providers.google);
  return <div className="ks-auth-providers">
    {(['github', 'google'] as const).map((provider) => <button key={provider} className={`ks-auth-provider ks-auth-provider-${provider}`} type="button"
      disabled={busy || pending !== null || !providers?.[provider]} onClick={() => void begin(provider)} aria-describedby={unavailable ? 'provider-availability' : undefined}>
      {pending === provider ? <LoaderCircle size={19} className="ks-auth-spinner" /> : provider === 'github' ? <img className="ks-auth-github-icon" src="/images/providers/github.svg" alt="" width={20} height={20} /> : <img src="/images/providers/google.png" alt="" width={20} height={20} />}
      <span>{pending === provider ? 'Connecting…' : `Continue with ${provider === 'github' ? 'GitHub' : 'Google'}`}</span>
    </button>)}
    {unavailable && <p id="provider-availability" className="ks-auth-provider-note">{!providers.github && !providers.google ? 'GitHub and Google sign-in are awaiting setup.' : `${!providers.github ? 'GitHub' : 'Google'} sign-in is awaiting setup.`}</p>}
    {error && <p className="ks-auth-error" role="alert">{error}</p>}
  </div>;
}
