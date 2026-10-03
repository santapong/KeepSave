import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { notifications } from '../api/teamVault';
import type { Notification } from '../api/coreTypes';
import { PageHeader } from '../components/cosmic/primitives';
export function NotificationsPage() {
  const [items, setItems] = useState<Notification[] | null>(null); const [error, setError] = useState('');
  useEffect(() => { let active = true; notifications().then(data => { if (active) setItems(data.notifications); }).catch(error => { if (active) setError(error instanceof Error ? error.message : 'Could not load reminders.'); }); return () => { active = false; }; }, []);
  return <div className="cz-page"><PageHeader eyebrow="Credential lifecycle" title="Renewal reminders" sub="KeepSave checks declared renewal and expiry dates at 30, 7 and 1 days. You only see reminders for credentials you currently own and can access." />{error && <p role="alert">{error}</p>}{items === null && !error && <p role="status">Loading reminders…</p>}{items?.length === 0 && <p>No current renewal reminders.</p>}<div className="ks-connections-list">{items?.map(item => <div className="ks-connections-row" key={item.id}><div><strong>{item.key}</strong><p>{item.threshold_days}-day reminder · Metadata revision {item.lifecycle_revision}</p><p>{new Date(item.created_at).toLocaleString()}</p></div><Link className="cz-btn" to={`/projects/${item.project_id}/lifecycle`}>Review lifecycle</Link></div>)}</div></div>;
}
