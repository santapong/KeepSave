import { useState, useEffect } from 'react';
import { Routes, Route, useNavigate, useLocation } from 'react-router-dom';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useToast } from '@/hooks/useToast';
import { cn } from '@/lib/utils';
import { Brain, GitCompare, ShieldAlert, BarChart3, Lightbulb, MessageSquare, Send, RefreshCw, CheckCircle, XCircle, AlertTriangle } from 'lucide-react';
import { listProjects } from '@/api/client';
import * as aiApi from '@/api/ai';
import type { Project } from '@/types';
import type { DriftCheck, Anomaly, UsageTrend, UsageForecast, SecretRecommendation, NLPQueryResult, AIProviderStatus } from '@/types/ai';

const TABS = [
  { key: '', label: 'Overview', icon: Brain },
  { key: 'drift', label: 'Drift Detection', icon: GitCompare },
  { key: 'anomalies', label: 'Anomalies', icon: ShieldAlert },
  { key: 'analytics', label: 'Analytics', icon: BarChart3 },
  { key: 'recommendations', label: 'Recommendations', icon: Lightbulb },
  { key: 'query', label: 'NLP Query', icon: MessageSquare },
] as const;

export function AIIntelligencePage() {
  const navigate = useNavigate();
  const location = useLocation();
  const currentTab = location.pathname.replace('/ai/', '').replace('/ai', '') || '';

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">AI Intelligence</h1>
          <p className="text-sm text-muted-foreground">Smart operations powered by multi-provider AI</p>
        </div>
      </div>
      <div className="flex gap-1 overflow-x-auto border-b border-border pb-px">
        {TABS.map((tab) => {
          const isActive = currentTab === tab.key;
          const Icon = tab.icon;
          return (
            <button key={tab.key} onClick={() => navigate(tab.key ? `/ai/${tab.key}` : '/ai')}
              className={cn('inline-flex items-center gap-2 px-4 py-2.5 text-sm font-medium whitespace-nowrap border-b-2 transition-colors',
                isActive ? 'border-primary text-primary' : 'border-transparent text-muted-foreground hover:text-foreground')}>
              <Icon className="h-4 w-4" />{tab.label}
            </button>
          );
        })}
      </div>
      <Routes>
        <Route index element={<OverviewTab />} />
        <Route path="drift" element={<DriftTab />} />
        <Route path="anomalies" element={<AnomaliesTab />} />
        <Route path="analytics" element={<AnalyticsTab />} />
        <Route path="recommendations" element={<RecommendationsTab />} />
        <Route path="query" element={<NLPQueryTab />} />
      </Routes>
    </div>
  );
}

// --- Overview Tab ---
function OverviewTab() {
  const [providers, setProviders] = useState<AIProviderStatus[]>([]);
  const [hasProvider, setHasProvider] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    aiApi.listAIProviders().then((d) => { setProviders(d.providers || []); setHasProvider(d.has_provider); }).finally(() => setLoading(false));
  }, []);

  if (loading) return <Skeleton className="h-48 w-full" />;

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader><CardTitle>AI Provider Status</CardTitle></CardHeader>
        <CardContent>
          {!hasProvider && <p className="text-muted-foreground mb-4">No AI providers configured. Set one of: ANTHROPIC_API_KEY, OPENAI_API_KEY, GOOGLE_API_KEY, GROQ_API_KEY, MISTRAL_API_KEY, or OLLAMA_BASE_URL</p>}
          {providers.length === 0 && <p className="text-muted-foreground">No providers registered.</p>}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {providers.map((p) => (
              <Card key={p.provider}>
                <CardContent className="pt-4">
                  <div className="flex items-center justify-between">
                    <span className="font-semibold capitalize">{p.provider}</span>
                    <Badge variant={p.available ? 'default' : 'secondary'}>{p.available ? 'Available' : 'Offline'}</Badge>
                  </div>
                  <p className="text-sm text-muted-foreground mt-1">Model: {p.model}</p>
                </CardContent>
              </Card>
            ))}
          </div>
        </CardContent>
      </Card>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {[
          { label: 'Drift Detection', desc: 'Compare secrets across environments', icon: GitCompare },
          { label: 'Anomaly Detection', desc: 'Z-score analysis of access patterns', icon: ShieldAlert },
          { label: 'Analytics & Forecasting', desc: 'Usage trends with linear regression', icon: BarChart3 },
          { label: 'Smart Recommendations', desc: 'AI-powered secret analysis', icon: Lightbulb },
        ].map((f) => (
          <Card key={f.label}><CardContent className="pt-4"><f.icon className="h-8 w-8 text-primary mb-2" /><h3 className="font-semibold">{f.label}</h3><p className="text-sm text-muted-foreground">{f.desc}</p></CardContent></Card>
        ))}
      </div>
    </div>
  );
}

// --- Drift Detection Tab ---
function DriftTab() {
  const { toast } = useToast();
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProject, setSelectedProject] = useState('');
  const [sourceEnv, setSourceEnv] = useState('alpha');
  const [targetEnv, setTargetEnv] = useState('uat');
  const [checks, setChecks] = useState<DriftCheck[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => { listProjects().then(setProjects); }, []);
  useEffect(() => { if (selectedProject) aiApi.listDriftChecks(selectedProject).then(setChecks); }, [selectedProject]);

  const runDrift = async () => {
    if (!selectedProject) return;
    setLoading(true);
    try {
      const check = await aiApi.detectDrift(selectedProject, sourceEnv, targetEnv);
      setChecks((prev) => [check, ...prev]);
      toast({ title: 'Drift check completed', description: `Found ${check.drifted_keys} drifted keys` });
    } catch (e: unknown) { toast({ title: 'Error', description: (e as Error).message, variant: 'destructive' }); }
    finally { setLoading(false); }
  };

  return (
    <div className="space-y-4">
      <Card><CardHeader><CardTitle>Run Drift Check</CardTitle></CardHeader><CardContent>
        <div className="flex gap-3 flex-wrap items-end">
          <div><label className="text-sm font-medium">Project</label>
            <Select value={selectedProject} onValueChange={setSelectedProject}>
              <SelectTrigger className="w-[200px]"><SelectValue placeholder="Select project" /></SelectTrigger>
              <SelectContent>{projects.map((p) => <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>)}</SelectContent>
            </Select></div>
          <div><label className="text-sm font-medium">Source</label>
            <Select value={sourceEnv} onValueChange={setSourceEnv}>
              <SelectTrigger className="w-[120px]"><SelectValue /></SelectTrigger>
              <SelectContent><SelectItem value="alpha">Alpha</SelectItem><SelectItem value="uat">UAT</SelectItem><SelectItem value="prod">PROD</SelectItem></SelectContent>
            </Select></div>
          <div><label className="text-sm font-medium">Target</label>
            <Select value={targetEnv} onValueChange={setTargetEnv}>
              <SelectTrigger className="w-[120px]"><SelectValue /></SelectTrigger>
              <SelectContent><SelectItem value="alpha">Alpha</SelectItem><SelectItem value="uat">UAT</SelectItem><SelectItem value="prod">PROD</SelectItem></SelectContent>
            </Select></div>
          <Button onClick={runDrift} disabled={loading || !selectedProject}>{loading ? <><RefreshCw className="h-4 w-4 mr-2 animate-spin" />Running...</> : 'Detect Drift'}</Button>
        </div>
      </CardContent></Card>
      {checks.length > 0 && <Card><CardHeader><CardTitle>Drift History</CardTitle></CardHeader><CardContent>
        <Table><TableHeader><TableRow><TableHead>Date</TableHead><TableHead>Environments</TableHead><TableHead>Total Keys</TableHead><TableHead>Drifted</TableHead><TableHead>Status</TableHead></TableRow></TableHeader>
          <TableBody>{checks.map((c) => (
            <TableRow key={c.id}><TableCell>{new Date(c.created_at).toLocaleString()}</TableCell><TableCell>{c.source_env} → {c.target_env}</TableCell><TableCell>{c.total_keys}</TableCell>
              <TableCell><Badge variant={c.drifted_keys > 0 ? 'destructive' : 'default'}>{c.drifted_keys}</Badge></TableCell><TableCell><Badge>{c.status}</Badge></TableCell></TableRow>
          ))}</TableBody></Table>
        {checks[0]?.remediation && <div className="mt-4 p-4 bg-muted rounded-lg"><h4 className="font-semibold mb-2">AI Remediation Plan</h4><p className="text-sm whitespace-pre-wrap">{checks[0].remediation}</p></div>}
      </CardContent></Card>}
    </div>
  );
}

// --- Anomalies Tab ---
function AnomaliesTab() {
  const { toast } = useToast();
  const [anomalies, setAnomalies] = useState<Anomaly[]>([]);
  const [loading, setLoading] = useState(true);

  const load = () => { setLoading(true); aiApi.listAnomalies().then(setAnomalies).finally(() => setLoading(false)); };
  useEffect(load, []);

  const handleAck = async (id: string) => {
    await aiApi.acknowledgeAnomaly(id); toast({ title: 'Anomaly acknowledged' }); load();
  };
  const handleResolve = async (id: string) => {
    await aiApi.resolveAnomaly(id); toast({ title: 'Anomaly resolved' }); load();
  };

  if (loading) return <Skeleton className="h-48 w-full" />;

  return (
    <Card><CardHeader><CardTitle>Detected Anomalies</CardTitle></CardHeader><CardContent>
      {anomalies.length === 0 ? <p className="text-muted-foreground">No anomalies detected.</p> : (
        <Table><TableHeader><TableRow><TableHead>Detected</TableHead><TableHead>Type</TableHead><TableHead>Severity</TableHead><TableHead>Description</TableHead><TableHead>Status</TableHead><TableHead>Actions</TableHead></TableRow></TableHeader>
          <TableBody>{anomalies.map((a) => (
            <TableRow key={a.id}><TableCell className="text-sm">{new Date(a.detected_at).toLocaleString()}</TableCell>
              <TableCell><Badge variant="outline">{a.anomaly_type}</Badge></TableCell>
              <TableCell><Badge variant={a.severity === 'critical' ? 'destructive' : a.severity === 'high' ? 'destructive' : 'secondary'}>{a.severity}</Badge></TableCell>
              <TableCell className="text-sm max-w-md truncate">{a.description}</TableCell>
              <TableCell><Badge>{a.status}</Badge></TableCell>
              <TableCell className="flex gap-1">
                {a.status === 'open' && <Button size="sm" variant="outline" onClick={() => handleAck(a.id)}><CheckCircle className="h-3 w-3" /></Button>}
                {(a.status === 'open' || a.status === 'acknowledged') && <Button size="sm" variant="outline" onClick={() => handleResolve(a.id)}><XCircle className="h-3 w-3" /></Button>}
              </TableCell></TableRow>
          ))}</TableBody></Table>)}
    </CardContent></Card>
  );
}

// --- Analytics Tab ---
function AnalyticsTab() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProject, setSelectedProject] = useState('');
  const [trends, setTrends] = useState<UsageTrend[]>([]);
  const [forecasts, setForecasts] = useState<UsageForecast[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => { listProjects().then(setProjects); }, []);
  useEffect(() => {
    if (!selectedProject) return;
    setLoading(true);
    Promise.all([aiApi.getUsageTrends(selectedProject), aiApi.getUsageForecast(selectedProject)])
      .then(([t, f]) => { setTrends(t); setForecasts(f); }).finally(() => setLoading(false));
  }, [selectedProject]);

  return (
    <div className="space-y-4">
      <div><Select value={selectedProject} onValueChange={setSelectedProject}>
        <SelectTrigger className="w-[250px]"><SelectValue placeholder="Select project" /></SelectTrigger>
        <SelectContent>{projects.map((p) => <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>)}</SelectContent>
      </Select></div>
      {loading ? <Skeleton className="h-48 w-full" /> : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <Card><CardHeader><CardTitle>Usage Trends</CardTitle></CardHeader><CardContent>
            {trends.length === 0 ? <p className="text-muted-foreground">No usage data yet.</p> : (
              <Table><TableHeader><TableRow><TableHead>Date</TableHead><TableHead>Accesses</TableHead><TableHead>Growth</TableHead></TableRow></TableHeader>
                <TableBody>{trends.map((t, i) => (
                  <TableRow key={i}><TableCell>{t.date}</TableCell><TableCell>{t.access_count}</TableCell>
                    <TableCell>{t.growth_percent > 0 ? <span className="text-green-600">+{t.growth_percent.toFixed(1)}%</span> : t.growth_percent < 0 ? <span className="text-red-600">{t.growth_percent.toFixed(1)}%</span> : '—'}</TableCell></TableRow>
                ))}</TableBody></Table>)}
          </CardContent></Card>
          <Card><CardHeader><CardTitle>Forecast (14 days)</CardTitle></CardHeader><CardContent>
            {forecasts.length === 0 ? <p className="text-muted-foreground">Not enough data to forecast.</p> : (
              <Table><TableHeader><TableRow><TableHead>Date</TableHead><TableHead>Predicted</TableHead><TableHead>Range</TableHead><TableHead>Confidence</TableHead></TableRow></TableHeader>
                <TableBody>{forecasts.map((f, i) => (
                  <TableRow key={i}><TableCell>{f.date}</TableCell><TableCell>{f.predicted_count.toFixed(0)}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">{f.lower_bound.toFixed(0)} – {f.upper_bound.toFixed(0)}</TableCell>
                    <TableCell><Badge variant={f.confidence > 0.7 ? 'default' : 'secondary'}>{(f.confidence * 100).toFixed(0)}%</Badge></TableCell></TableRow>
                ))}</TableBody></Table>)}
          </CardContent></Card>
        </div>)}
    </div>
  );
}

// --- Recommendations Tab ---
function RecommendationsTab() {
  const { toast } = useToast();
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProject, setSelectedProject] = useState('');
  const [recs, setRecs] = useState<SecretRecommendation[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => { listProjects().then(setProjects); }, []);
  useEffect(() => { if (selectedProject) aiApi.listRecommendations(selectedProject).then(setRecs); }, [selectedProject]);

  const generate = async () => {
    if (!selectedProject) return;
    setLoading(true);
    try {
      const newRecs = await aiApi.generateRecommendations(selectedProject);
      setRecs(newRecs);
      toast({ title: 'Recommendations generated', description: `Found ${newRecs.length} recommendations` });
    } catch (e: unknown) { toast({ title: 'Error', description: (e as Error).message, variant: 'destructive' }); }
    finally { setLoading(false); }
  };

  const dismiss = async (id: string) => {
    await aiApi.dismissRecommendation(selectedProject, id);
    setRecs((prev) => prev.filter((r) => r.id !== id));
    toast({ title: 'Recommendation dismissed' });
  };

  const severityIcon = (s: string) => {
    if (s === 'critical') return <AlertTriangle className="h-4 w-4 text-red-500" />;
    if (s === 'warning') return <AlertTriangle className="h-4 w-4 text-yellow-500" />;
    return <CheckCircle className="h-4 w-4 text-blue-500" />;
  };

  return (
    <div className="space-y-4">
      <div className="flex gap-3 items-end">
        <Select value={selectedProject} onValueChange={setSelectedProject}>
          <SelectTrigger className="w-[250px]"><SelectValue placeholder="Select project" /></SelectTrigger>
          <SelectContent>{projects.map((p) => <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>)}</SelectContent>
        </Select>
        <Button onClick={generate} disabled={loading || !selectedProject}>{loading ? <><RefreshCw className="h-4 w-4 mr-2 animate-spin" />Analyzing...</> : 'Generate Recommendations'}</Button>
      </div>
      {recs.length > 0 && <div className="space-y-3">
        {recs.map((r) => (
          <Card key={r.id}><CardContent className="pt-4">
            <div className="flex items-start justify-between">
              <div className="flex items-start gap-3">
                {severityIcon(r.severity)}
                <div><h4 className="font-semibold">{r.title}</h4><p className="text-sm text-muted-foreground mt-1">{r.description}</p>
                  {r.suggested_action && <p className="text-sm mt-2"><strong>Action:</strong> {r.suggested_action}</p>}
                  {r.affected_keys?.length > 0 && <div className="flex gap-1 mt-2">{r.affected_keys.map((k) => <Badge key={k} variant="outline">{k}</Badge>)}</div>}
                </div>
              </div>
              <Button size="sm" variant="ghost" onClick={() => dismiss(r.id)}>Dismiss</Button>
            </div>
          </CardContent></Card>
        ))}
      </div>}
    </div>
  );
}

// --- NLP Query Tab ---
function NLPQueryTab() {
  const [query, setQuery] = useState('');
  const [result, setResult] = useState<NLPQueryResult | null>(null);
  const [loading, setLoading] = useState(false);

  const handleQuery = async () => {
    if (!query.trim()) return;
    setLoading(true);
    try { const r = await aiApi.nlpQuery(query); setResult(r); } catch { setResult(null); }
    finally { setLoading(false); }
  };

  return (
    <div className="space-y-4">
      <Card><CardHeader><CardTitle>Ask about your secrets</CardTitle></CardHeader><CardContent>
        <div className="flex gap-3">
          <Input value={query} onChange={(e) => setQuery(e.target.value)} placeholder='e.g. "Where is the database URL for production?"'
            onKeyDown={(e) => { if (e.key === 'Enter') handleQuery(); }} className="flex-1" />
          <Button onClick={handleQuery} disabled={loading || !query.trim()}>{loading ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}</Button>
        </div>
      </CardContent></Card>
      {result && <Card><CardHeader><CardTitle>Result</CardTitle></CardHeader><CardContent>
        <div className="space-y-4">
          <div className="flex gap-2 items-center">
            <Badge>{result.intent}</Badge>
            <span className="text-sm text-muted-foreground">via {result.provider} ({result.model})</span>
          </div>
          <p>{result.explanation}</p>
          {result.matched_secrets?.length > 0 && <div>
            <h4 className="font-semibold mb-2">Matched Secrets</h4>
            <Table><TableHeader><TableRow><TableHead>Project</TableHead><TableHead>Environment</TableHead><TableHead>Key</TableHead><TableHead>Score</TableHead><TableHead>Reason</TableHead></TableRow></TableHeader>
              <TableBody>{result.matched_secrets.map((m, i) => (
                <TableRow key={i}><TableCell>{m.project_name}</TableCell><TableCell><Badge variant="outline">{m.environment}</Badge></TableCell>
                  <TableCell className="font-mono">{m.key}</TableCell><TableCell>{(m.score * 100).toFixed(0)}%</TableCell><TableCell className="text-sm">{m.reason}</TableCell></TableRow>
              ))}</TableBody></Table>
          </div>}
          {result.suggestions?.length > 0 && <div><h4 className="font-semibold mb-2">Suggestions</h4>
            <ul className="list-disc list-inside text-sm">{result.suggestions.map((s, i) => <li key={i}>{s}</li>)}</ul></div>}
        </div>
      </CardContent></Card>}
    </div>
  );
}
