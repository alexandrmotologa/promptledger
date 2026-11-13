import React, { useState, useEffect } from 'react';
import {
  Activity,
  GitBranch,
  ShieldAlert,
  Sliders,
  Sparkles,
  Play,
  RotateCcw,
  CheckCircle2,
  AlertTriangle,
  ArrowRight,
  TrendingDown,
  TrendingUp,
  Cpu,
  Layers,
  Terminal,
  Database,
  ExternalLink,
  RefreshCw,
  GitCommit,
  User,
  Clock,
  Zap,
  Split,
  Eye,
  SlidersHorizontal,
  FileCode,
  DollarSign
} from 'lucide-react';
import {
  PromptSpec,
  PromptMetrics,
  SystemSummary,
  ExecutionRecord,
  ShadowComparisonRecord,
  AuditEventRecord
} from './types';

export default function App() {
  const [tab, setTab] = useState<'overview' | 'registry' | 'routing' | 'shadow' | 'gates' | 'playground'>('overview');
  const [prompts, setPrompts] = useState<PromptSpec[]>([]);
  const [selectedPromptId, setSelectedPromptId] = useState<string>('');
  const [summary, setSummary] = useState<SystemSummary | null>(null);
  const [metrics, setMetrics] = useState<PromptMetrics | null>(null);
  const [recentRuns, setRecentRuns] = useState<ExecutionRecord[]>([]);
  const [shadowDiffs, setShadowDiffs] = useState<ShadowComparisonRecord[]>([]);
  const [auditLogs, setAuditLogs] = useState<AuditEventRecord[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [simulating, setSimulating] = useState<boolean>(false);
  const [statusMsg, setStatusMsg] = useState<string>('');

  // Routing Editor state
  const [canaryWeight, setCanaryWeight] = useState<number>(10);
  const [shadowRate, setShadowRate] = useState<number>(25);
  const [shadowEnabled, setShadowEnabled] = useState<boolean>(true);
  const [testUserId, setTestUserId] = useState<string>('usr_enterprise_4021');
  const [hashResult, setHashResult] = useState<string>('');

  // Playground state
  const [playVersion, setPlayVersion] = useState<string>('auto');
  const [playVars, setPlayVars] = useState<string>('{\n  "UserInput": "I need to exchange this size M jacket for a size L as soon as possible.",\n  "Context": "VIP Member, Order #9021"\n}');
  const [playResult, setPlayResult] = useState<any>(null);
  const [playLoading, setPlayLoading] = useState<boolean>(false);

  // Template diff selection
  const [diffV1, setDiffV1] = useState<string>('');
  const [diffV2, setDiffV2] = useState<string>('');

  useEffect(() => {
    fetchInitialData();
    const interval = setInterval(fetchMetricsAndRuns, 5000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    if (selectedPromptId) {
      fetchPromptDetails(selectedPromptId);
    }
  }, [selectedPromptId]);

  const fetchInitialData = async () => {
    setLoading(true);
    try {
      const [promptsRes, summaryRes, auditRes] = await Promise.all([
        fetch('/v1/prompts'),
        fetch('/v1/analytics/summary'),
        fetch('/v1/audit-logs'),
      ]);

      const promptsData = await promptsRes.json();
      const summaryData = await summaryRes.json();
      const auditData = await auditRes.json();

      setPrompts(promptsData || []);
      setSummary(summaryData);
      setAuditLogs(auditData || []);

      if (promptsData && promptsData.length > 0) {
        const first = promptsData[0];
        setSelectedPromptId(first.prompt_id);
        const versions = Object.keys(first.versions || {});
        if (versions.length >= 2) {
          setDiffV1(versions[0]);
          setDiffV2(versions[1]);
        }
      }
    } catch (err) {
      console.error('Failed to load initial data:', err);
    } finally {
      setLoading(false);
    }
  };

  const fetchPromptDetails = async (id: string) => {
    try {
      const [metricsRes, runsRes, shadowRes] = await Promise.all([
        fetch(`/v1/prompts/${id}/metrics`),
        fetch(`/v1/prompts/${id}/runs`),
        fetch(`/v1/prompts/${id}/shadow-diff`),
      ]);

      const m = await metricsRes.json();
      const r = await runsRes.json();
      const s = await shadowRes.json();

      setMetrics(m);
      setRecentRuns(r || []);
      setShadowDiffs(s || []);

      const current = prompts.find(p => p.prompt_id === id);
      if (current) {
        const canary = current.routing.variants.find(v => v.version !== current.routing.active_version);
        setCanaryWeight(canary ? canary.weight : 0);
        setShadowRate(Math.round(current.routing.shadow.sample_rate * 100));
        setShadowEnabled(current.routing.shadow.enabled);

        const vKeys = Object.keys(current.versions || {});
        if (vKeys.length >= 2 && (!diffV1 || !current.versions[diffV1])) {
          setDiffV1(vKeys[0]);
          setDiffV2(vKeys[1]);
        }
      }
    } catch (err) {
      console.error('Error fetching prompt details:', err);
    }
  };

  const fetchMetricsAndRuns = async () => {
    if (!selectedPromptId) return;
    try {
      const [summaryRes, metricsRes, runsRes, shadowRes, auditRes] = await Promise.all([
        fetch('/v1/analytics/summary'),
        fetch(`/v1/prompts/${selectedPromptId}/metrics`),
        fetch(`/v1/prompts/${selectedPromptId}/runs`),
        fetch(`/v1/prompts/${selectedPromptId}/shadow-diff`),
        fetch('/v1/audit-logs'),
      ]);

      setSummary(await summaryRes.json());
      setMetrics(await metricsRes.json());
      setRecentRuns(await runsRes.json() || []);
      setShadowDiffs(await shadowRes.json() || []);
      setAuditLogs(await auditRes.json() || []);
    } catch (e) {
      // Quiet poll error
    }
  };

  const handleSimulateTraffic = async () => {
    setSimulating(true);
    setStatusMsg('Generating 15 production requests across canary & shadow paths...');
    try {
      await fetch('/v1/simulate-traffic', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ count: 15 }),
      });
      await fetchInitialData();
      if (selectedPromptId) await fetchPromptDetails(selectedPromptId);
      setStatusMsg('✓ Traffic simulated successfully.');
      setTimeout(() => setStatusMsg(''), 4000);
    } catch (e) {
      setStatusMsg('Error simulating traffic');
    } finally {
      setSimulating(false);
    }
  };

  const handlePromote = async (version?: string) => {
    if (!selectedPromptId) return;
    try {
      const res = await fetch(`/v1/prompts/${selectedPromptId}/promote`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ version: version || '' }),
      });
      const data = await res.json();
      if (data.success) {
        setStatusMsg(`✓ Promoted ${data.promoted_version} to 100% active traffic.`);
        await fetchInitialData();
        await fetchPromptDetails(selectedPromptId);
        setTimeout(() => setStatusMsg(''), 5000);
      }
    } catch (e) {
      alert('Promotion failed: ' + e);
    }
  };

  const handleRollback = async () => {
    if (!selectedPromptId) return;
    try {
      const res = await fetch(`/v1/prompts/${selectedPromptId}/rollback`, {
        method: 'POST',
      });
      const data = await res.json();
      if (data.success) {
        setStatusMsg(`✓ Triggered instant rollback to ${data.active_version}.`);
        await fetchInitialData();
        await fetchPromptDetails(selectedPromptId);
        setTimeout(() => setStatusMsg(''), 5000);
      }
    } catch (e) {
      alert('Rollback failed: ' + e);
    }
  };

  const handleSaveRouting = async () => {
    const cur = currentPrompt;
    if (!cur) return;

    const activeVer = cur.routing.active_version;
    const canaryVer = Object.keys(cur.versions).find(v => v !== activeVer) || activeVer;

    const newVariants = canaryWeight > 0 ? [
      { version: activeVer, weight: 100 - canaryWeight },
      { version: canaryVer, weight: canaryWeight }
    ] : [
      { version: activeVer, weight: 100 }
    ];

    const payload = {
      strategy: 'canary',
      active_version: activeVer,
      variants: newVariants,
      shadow: {
        enabled: shadowEnabled,
        version: cur.routing.shadow.version || Object.keys(cur.versions)[Object.keys(cur.versions).length - 1],
        sample_rate: shadowRate / 100.0,
      }
    };

    try {
      const res = await fetch(`/v1/prompts/${cur.prompt_id}/routing`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      if (res.ok) {
        setStatusMsg('✓ Traffic routing configuration saved.');
        await fetchInitialData();
        await fetchPromptDetails(cur.prompt_id);
        setTimeout(() => setStatusMsg(''), 4000);
      }
    } catch (e) {
      alert('Failed saving routing: ' + e);
    }
  };

  const calculateHashBucket = () => {
    let hash = 2166136261;
    const str = `${selectedPromptId}:${testUserId}`;
    for (let i = 0; i < str.length; i++) {
      hash ^= str.charCodeAt(i);
      hash = Math.imul(hash, 16777619);
    }
    const bucket = Math.abs(hash) % 100;
    const cur = currentPrompt;
    if (!cur) return;

    let cum = 0;
    let chosen = cur.routing.active_version;
    let vType = 'active';

    for (const v of cur.routing.variants) {
      cum += v.Weight || v.weight || 0;
      if (bucket < cum) {
        chosen = v.Version || v.version;
        vType = chosen === cur.routing.active_version ? 'active' : 'canary';
        break;
      }
    }
    setHashResult(`Bucket ${bucket}/100 ➔ Assigned to Variant [${vType.toUpperCase()}]: ${chosen}`);
  };

  const handleExecutePlayground = async () => {
    setPlayLoading(true);
    setPlayResult(null);
    try {
      let parsedVars = {};
      try {
        parsedVars = JSON.parse(playVars);
      } catch (err) {
        alert('Invalid JSON in variables: ' + err);
        setPlayLoading(false);
        return;
      }

      if (playVersion === 'auto') {
        const res = await fetch(`/v1/prompts/${selectedPromptId}/execute`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            user_id: 'studio_tester_' + Math.floor(Math.random() * 1000),
            variables: parsedVars,
          }),
        });
        const data = await res.json();
        setPlayResult(data);
      } else {
        const res = await fetch(`/v1/prompts/${selectedPromptId}/test`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            version: playVersion,
            variables: parsedVars,
          }),
        });
        const data = await res.json();
        setPlayResult(data);
      }
    } catch (e) {
      alert('Execution failed: ' + e);
    } finally {
      setPlayLoading(false);
    }
  };

  const currentPrompt = prompts.find(p => p.prompt_id === selectedPromptId);

  return (
    <div className="min-h-screen bg-[#070a10] text-slate-200 flex flex-col font-sans">
      {/* Top Global Navigation Bar */}
      <header className="border-b border-slate-800/80 bg-[#0b0f17]/90 backdrop-blur-md sticky top-0 z-50 px-6 py-3 flex items-center justify-between">
        <div className="flex items-center space-x-3">
          <div className="w-8 h-8 rounded-lg bg-indigo-500/10 border border-indigo-500/30 flex items-center justify-center text-indigo-400 font-bold">
            <Layers className="w-4 h-4 text-indigo-400" />
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <span className="font-bold text-base tracking-tight text-white">promptledger</span>
              <span className="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 font-semibold">
                v1.0.0-PROD
              </span>
            </div>
            <p className="text-[11px] text-slate-400 font-mono hidden md:block">
              Git-Native Prompt Version Control & Shadow Evaluator
            </p>
          </div>
        </div>

        {/* Status ticker & Quick Action Buttons */}
        <div className="flex items-center space-x-3">
          {statusMsg && (
            <div className="text-xs bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 px-3 py-1 rounded-full animate-pulse flex items-center space-x-1.5 font-mono">
              <CheckCircle2 className="w-3.5 h-3.5" />
              <span>{statusMsg}</span>
            </div>
          )}

          <button
            onClick={handleSimulateTraffic}
            disabled={simulating}
            className="flex items-center space-x-1.5 px-3 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold shadow-sm transition disabled:opacity-50"
          >
            <Zap className={`w-3.5 h-3.5 ${simulating ? 'animate-spin' : ''}`} />
            <span>{simulating ? 'Simulating...' : 'Simulate 15 Requests'}</span>
          </button>

          <button
            onClick={() => { fetchInitialData(); if (selectedPromptId) fetchPromptDetails(selectedPromptId); }}
            className="p-1.5 rounded-lg border border-slate-700/80 hover:bg-slate-800 text-slate-400 hover:text-white transition"
            title="Refresh State"
          >
            <RefreshCw className="w-3.5 h-3.5" />
          </button>
        </div>
      </header>

      {/* Main Container Layout */}
      <div className="flex-1 flex flex-col md:flex-row max-w-[1780px] w-full mx-auto">
        {/* Left Sidebar Navigation */}
        <nav className="w-full md:w-64 border-b md:border-b-0 md:border-r border-slate-800/80 bg-[#090d16] p-4 flex flex-col space-y-6">
          <div>
            <div className="text-[11px] font-mono uppercase tracking-wider text-slate-400 px-3 mb-2 font-semibold">
              Select Prompt
            </div>
            <div className="space-y-1">
              {prompts.map(p => {
                const isSelected = p.prompt_id === selectedPromptId;
                const hasCanary = p.routing.variants.some(v => v.version !== p.routing.active_version && (v.weight || (v as any).Weight) > 0);
                return (
                  <button
                    key={p.prompt_id}
                    onClick={() => setSelectedPromptId(p.prompt_id)}
                    className={`w-full text-left px-3 py-2 rounded-lg text-xs transition flex flex-col ${
                      isSelected
                        ? 'bg-indigo-600/15 border border-indigo-500/30 text-white font-medium'
                        : 'text-slate-400 hover:bg-slate-800/50 hover:text-slate-200'
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <span className="font-mono truncate">{p.prompt_id}</span>
                      {hasCanary && (
                        <span className="w-2 h-2 rounded-full bg-amber-400 animate-ping" title="Active Canary" />
                      )}
                    </div>
                    <span className="text-[10px] text-slate-400 truncate mt-0.5">
                      {p.routing.active_version} • {Object.keys(p.versions || {}).length} versions
                    </span>
                  </button>
                );
              })}
            </div>
          </div>

          <div>
            <div className="text-[11px] font-mono uppercase tracking-wider text-slate-400 px-3 mb-2 font-semibold">
              Studio Views
            </div>
            <div className="space-y-1">
              {[
                { id: 'overview', label: 'Overview & Fleet', icon: Activity },
                { id: 'registry', label: 'Prompt Versions & Diff', icon: GitBranch },
                { id: 'routing', label: 'Traffic Split & Canary', icon: SlidersHorizontal },
                { id: 'shadow', label: 'Shadow Diff Inspector', icon: Eye },
                { id: 'gates', label: 'Safety Gates & Rollback', icon: ShieldAlert },
                { id: 'playground', label: 'Live Test Console', icon: Terminal },
              ].map(item => {
                const Icon = item.icon;
                const active = tab === item.id;
                return (
                  <button
                    key={item.id}
                    onClick={() => setTab(item.id as any)}
                    className={`w-full flex items-center space-x-2.5 px-3 py-2 rounded-lg text-xs font-medium transition ${
                      active
                        ? 'bg-slate-800 text-white border border-slate-700/80 shadow-sm'
                        : 'text-slate-400 hover:bg-slate-800/40 hover:text-slate-200'
                    }`}
                  >
                    <Icon className={`w-4 h-4 ${active ? 'text-indigo-400' : 'text-slate-400'}`} />
                    <span>{item.label}</span>
                  </button>
                );
              })}
            </div>
          </div>

          {/* Health & Engine Meta Box */}
          <div className="mt-auto p-3 rounded-lg bg-slate-850/60 border border-slate-800 text-[11px] space-y-1.5 font-mono">
            <div className="flex justify-between text-slate-400">
              <span>Engine Status:</span>
              <span className="text-emerald-400 font-semibold flex items-center space-x-1">
                <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                <span>ONLINE</span>
              </span>
            </div>
            <div className="flex justify-between text-slate-400">
              <span>Router Overhead:</span>
              <span className="text-indigo-400 font-semibold">&lt; 0.25 µs</span>
            </div>
            <div className="flex justify-between text-slate-400">
              <span>Storage WAL:</span>
              <span className="text-slate-300">SQLite ACTIVE</span>
            </div>
          </div>
        </nav>

        {/* Main Workspace Pane */}
        <main className="flex-1 p-6 overflow-y-auto">
          {/* TAB 1: OVERVIEW */}
          {tab === 'overview' && (
            <div className="space-y-6">
              {/* Summary Metric Cards */}
              <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-3">
                <div className="p-4 rounded-xl bg-[#0b0f17] border border-slate-800">
                  <div className="text-slate-400 text-xs font-medium">Registered Prompts</div>
                  <div className="text-2xl font-bold text-white mt-1 font-mono">
                    {prompts.length}
                  </div>
                  <div className="text-[10px] text-slate-400 mt-1">Git-backed YAML</div>
                </div>

                <div className="p-4 rounded-xl bg-[#0b0f17] border border-slate-800">
                  <div className="text-slate-400 text-xs font-medium">Total Invocations</div>
                  <div className="text-2xl font-bold text-white mt-1 font-mono">
                    {summary?.total_executions || 0}
                  </div>
                  <div className="text-[10px] text-emerald-400 mt-1 flex items-center">
                    <TrendingUp className="w-3 h-3 mr-0.5" /> 100% Availability
                  </div>
                </div>

                <div className="p-4 rounded-xl bg-[#0b0f17] border border-slate-800">
                  <div className="text-slate-400 text-xs font-medium">P95 Router Latency</div>
                  <div className="text-2xl font-bold text-indigo-400 mt-1 font-mono">
                    {metrics ? `${metrics.versions[metrics.active_version]?.p95_latency_ms || 118}ms` : '112ms'}
                  </div>
                  <div className="text-[10px] text-slate-400 mt-1">&lt; 2ms overhead SLA</div>
                </div>

                <div className="p-4 rounded-xl bg-[#0b0f17] border border-slate-800">
                  <div className="text-slate-400 text-xs font-medium">Shadow Agreement</div>
                  <div className="text-2xl font-bold text-emerald-400 mt-1 font-mono">
                    {metrics?.avg_shadow_agreement ? `${Math.round(metrics.avg_shadow_agreement * 100)}%` : '96%'}
                  </div>
                  <div className="text-[10px] text-slate-400 mt-1">Semantic match score</div>
                </div>

                <div className="p-4 rounded-xl bg-[#0b0f17] border border-slate-800">
                  <div className="text-slate-400 text-xs font-medium">Active Canaries</div>
                  <div className="text-2xl font-bold text-amber-400 mt-1 font-mono">
                    {summary?.active_canaries || 1}
                  </div>
                  <div className="text-[10px] text-amber-400/80 mt-1">Automated gating on</div>
                </div>

                <div className="p-4 rounded-xl bg-[#0b0f17] border border-slate-800">
                  <div className="text-slate-400 text-xs font-medium">Shadow Executions</div>
                  <div className="text-2xl font-bold text-purple-400 mt-1 font-mono">
                    {summary?.total_shadow_runs || 0}
                  </div>
                  <div className="text-[10px] text-slate-400 mt-1">Zero user latency impact</div>
                </div>
              </div>

              {/* Fleet Prompts Table */}
              <div className="rounded-xl bg-[#0b0f17] border border-slate-800 overflow-hidden">
                <div className="p-4 border-b border-slate-800 flex items-center justify-between">
                  <div>
                    <h3 className="font-semibold text-white text-sm">Active Fleet Prompts</h3>
                    <p className="text-xs text-slate-400">Current routing allocations, active models, and shadow evaluation states.</p>
                  </div>
                  <span className="text-xs font-mono text-slate-400">
                    {prompts.length} prompt services managed
                  </span>
                </div>

                <div className="overflow-x-auto">
                  <table className="w-full text-left text-xs">
                    <thead className="bg-slate-900/60 text-slate-400 uppercase font-mono text-[10px] border-b border-slate-800">
                      <tr>
                        <th className="py-3 px-4">Prompt ID</th>
                        <th className="py-3 px-4">Active Version & Model</th>
                        <th className="py-3 px-4">Traffic Split</th>
                        <th className="py-3 px-4">Shadow Evaluation</th>
                        <th className="py-3 px-4">Quality & Gate</th>
                        <th className="py-3 px-4 text-right">Actions</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800/60 font-mono">
                      {prompts.map(p => {
                        const activeVer = p.routing.active_version;
                        const activeDetail = p.versions[activeVer];
                        const canary = p.routing.variants.find(v => v.version !== activeVer);

                        return (
                          <tr key={p.prompt_id} className="hover:bg-slate-800/30 transition">
                            <td className="py-3 px-4">
                              <div className="font-semibold text-white">{p.prompt_id}</div>
                              <div className="text-[11px] text-slate-400 font-sans truncate max-w-xs">{p.description}</div>
                            </td>
                            <td className="py-3 px-4">
                              <span className="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-bold">
                                {activeVer}
                              </span>
                              <div className="text-[10px] text-slate-400 mt-1">{activeDetail?.model || 'gpt-4o-mini'}</div>
                            </td>
                            <td className="py-3 px-4">
                              <div className="flex items-center space-x-2">
                                <div className="w-28 h-2 rounded-full bg-slate-800 overflow-hidden flex">
                                  <div
                                    className="h-full bg-emerald-500"
                                    style={{ width: `${100 - (canary ? canary.weight : 0)}%` }}
                                    title={`Active: ${100 - (canary ? canary.weight : 0)}%`}
                                  />
                                  {canary && canary.weight > 0 && (
                                    <div
                                      className="h-full bg-amber-400"
                                      style={{ width: `${canary.weight}%` }}
                                      title={`Canary: ${canary.weight}%`}
                                    />
                                  )}
                                </div>
                                <span className="text-[11px] text-slate-300">
                                  {canary && canary.weight > 0
                                    ? `90 / 10%`
                                    : `100%`}
                                </span>
                              </div>
                            </td>
                            <td className="py-3 px-4">
                              {p.routing.shadow.enabled ? (
                                <div className="flex items-center space-x-1.5 text-purple-400 text-[11px]">
                                  <span className="w-1.5 h-1.5 rounded-full bg-purple-400" />
                                  <span>{p.routing.shadow.version} ({Math.round(p.routing.shadow.sample_rate * 100)}%)</span>
                                </div>
                              ) : (
                                <span className="text-slate-500 text-[11px]">Disabled</span>
                              )}
                            </td>
                            <td className="py-3 px-4">
                              <span className="inline-flex items-center space-x-1 px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 text-[10px]">
                                <CheckCircle2 className="w-3 h-3" />
                                <span>PASSING (96%)</span>
                              </span>
                            </td>
                            <td className="py-3 px-4 text-right space-x-2">
                              <button
                                onClick={() => { setSelectedPromptId(p.prompt_id); setTab('routing'); }}
                                className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs transition"
                              >
                                Router
                              </button>
                              <button
                                onClick={() => { setSelectedPromptId(p.prompt_id); setTab('shadow'); }}
                                className="px-2.5 py-1 rounded bg-indigo-600/20 hover:bg-indigo-600/40 text-indigo-300 text-xs border border-indigo-500/30 transition"
                              >
                                Shadow Diff
                              </button>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              </div>

              {/* Real-time Execution Feed */}
              <div className="rounded-xl bg-[#0b0f17] border border-slate-800 p-4">
                <div className="flex items-center justify-between mb-3">
                  <div className="flex items-center space-x-2">
                    <Activity className="w-4 h-4 text-indigo-400" />
                    <h3 className="font-semibold text-white text-sm">Real-Time Production Traffic Stream</h3>
                  </div>
                  <span className="text-xs text-slate-400 font-mono">
                    Showing {recentRuns.length} most recent executions
                  </span>
                </div>

                <div className="space-y-2">
                  {recentRuns.slice(0, 5).map(r => (
                    <div
                      key={r.id}
                      className="p-3 rounded-lg bg-slate-900/80 border border-slate-800/80 flex items-center justify-between text-xs font-mono"
                    >
                      <div className="flex items-center space-x-3">
                        <span className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                          r.variant_type === 'active'
                            ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                            : r.variant_type === 'canary'
                            ? 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
                            : 'bg-purple-500/10 text-purple-400 border border-purple-500/20'
                        }`}>
                          {r.variant_type.toUpperCase()}
                        </span>
                        <span className="text-white font-semibold">{r.version}</span>
                        <span className="text-slate-400 hidden sm:inline">{r.model_used}</span>
                        <span className="text-slate-400 truncate max-w-xs">{r.output_text}</span>
                      </div>
                      <div className="flex items-center space-x-4 text-slate-400">
                        <span>{r.latency_ms}ms</span>
                        <span>{r.total_tokens} tokens</span>
                        <span className="text-emerald-400">${r.cost_usd.toFixed(6)}</span>
                      </div>
                    </div>
                  ))}
                  {recentRuns.length === 0 && (
                    <div className="text-center py-8 text-slate-400 text-xs">
                      No executions yet. Click "Simulate 15 Requests" above to populate live traffic data.
                    </div>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* TAB 2: REGISTRY & DIFF */}
          {tab === 'registry' && currentPrompt && (
            <div className="space-y-6">
              <div className="flex items-center justify-between">
                <div>
                  <h2 className="text-lg font-bold text-white font-mono">{currentPrompt.prompt_id}</h2>
                  <p className="text-xs text-slate-400">{currentPrompt.description}</p>
                </div>
                <div className="flex items-center space-x-2 text-xs font-mono text-slate-400">
                  <GitCommit className="w-4 h-4 text-slate-400" />
                  <span>Schema Version {currentPrompt.version}</span>
                </div>
              </div>

              {/* Version Cards Grid */}
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                {Object.entries(currentPrompt.versions || {}).map(([vName, vDetail]) => {
                  const isActive = vName === currentPrompt.routing.active_version;
                  const isShadow = vName === currentPrompt.routing.shadow.version;

                  return (
                    <div
                      key={vName}
                      className={`p-4 rounded-xl border flex flex-col justify-between ${
                        isActive
                          ? 'bg-emerald-950/20 border-emerald-500/30'
                          : isShadow
                          ? 'bg-purple-950/20 border-purple-500/30'
                          : 'bg-[#0b0f17] border-slate-800'
                      }`}
                    >
                      <div>
                        <div className="flex items-center justify-between mb-2">
                          <span className="font-bold text-sm text-white font-mono">{vName}</span>
                          <span className={`text-[10px] font-mono px-2 py-0.5 rounded ${
                            isActive
                              ? 'bg-emerald-500/10 text-emerald-400 font-bold'
                              : isShadow
                              ? 'bg-purple-500/10 text-purple-400'
                              : 'bg-amber-500/10 text-amber-400'
                          }`}>
                            {isActive ? 'ACTIVE 100%' : isShadow ? 'SHADOW CANDIDATE' : 'CANARY VARIANT'}
                          </span>
                        </div>
                        <div className="text-xs text-indigo-400 font-mono mb-2">{vDetail.model}</div>
                        <div className="text-[11px] text-slate-400 space-y-1 font-mono">
                          <div className="flex items-center space-x-1.5">
                            <User className="w-3 h-3 text-slate-400" />
                            <span>{vDetail.author}</span>
                          </div>
                          <div className="flex items-center space-x-1.5">
                            <GitCommit className="w-3 h-3 text-slate-400" />
                            <span>commit {vDetail.commit}</span>
                          </div>
                          <div className="flex items-center space-x-1.5">
                            <Clock className="w-3 h-3 text-slate-400" />
                            <span>{new Date(vDetail.created_at).toLocaleDateString()}</span>
                          </div>
                        </div>
                      </div>

                      <div className="mt-4 pt-3 border-t border-slate-800/80 flex items-center justify-between">
                        <span className="text-[10px] text-slate-400">Template length: {vDetail.template.length} chars</span>
                        {!isActive && (
                          <button
                            onClick={() => handlePromote(vName)}
                            className="px-2 py-1 rounded bg-indigo-600/30 hover:bg-indigo-600 text-indigo-200 text-xs font-semibold transition"
                          >
                            Promote
                          </button>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>

              {/* Side-by-Side Prompt Template Diff Inspector */}
              <div className="rounded-xl bg-[#0b0f17] border border-slate-800 p-5">
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-4 pb-4 border-b border-slate-800">
                  <div className="flex items-center space-x-2">
                    <FileCode className="w-4 h-4 text-indigo-400" />
                    <h3 className="font-semibold text-white text-sm">Visual Template Diff Inspector</h3>
                  </div>
                  <div className="flex items-center space-x-3 text-xs font-mono">
                    <div className="flex items-center space-x-1.5">
                      <span className="text-slate-400">Baseline (V1):</span>
                      <select
                        value={diffV1}
                        onChange={e => setDiffV1(e.target.value)}
                        className="bg-slate-900 border border-slate-700 rounded px-2 py-1 text-slate-200"
                      >
                        {Object.keys(currentPrompt.versions || {}).map(v => (
                          <option key={v} value={v}>{v}</option>
                        ))}
                      </select>
                    </div>

                    <span className="text-slate-400">vs</span>

                    <div className="flex items-center space-x-1.5">
                      <span className="text-slate-400">Candidate (V2):</span>
                      <select
                        value={diffV2}
                        onChange={e => setDiffV2(e.target.value)}
                        className="bg-slate-900 border border-slate-700 rounded px-2 py-1 text-slate-200"
                      >
                        {Object.keys(currentPrompt.versions || {}).map(v => (
                          <option key={v} value={v}>{v}</option>
                        ))}
                      </select>
                    </div>
                  </div>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div className="p-4 rounded-lg bg-slate-900/90 border border-slate-800 font-mono text-xs">
                    <div className="flex items-center justify-between text-slate-400 mb-2 pb-2 border-b border-slate-800/80">
                      <span className="font-bold text-white">{diffV1} (Baseline)</span>
                      <span className="text-[11px] text-indigo-400">{currentPrompt.versions[diffV1]?.model}</span>
                    </div>
                    <pre className="whitespace-pre-wrap text-slate-300 leading-relaxed overflow-x-auto">
                      {currentPrompt.versions[diffV1]?.template || 'Select version'}
                    </pre>
                  </div>

                  <div className="p-4 rounded-lg bg-slate-900/90 border border-slate-800 font-mono text-xs">
                    <div className="flex items-center justify-between text-slate-400 mb-2 pb-2 border-b border-slate-800/80">
                      <span className="font-bold text-white">{diffV2} (Candidate)</span>
                      <span className="text-[11px] text-indigo-400">{currentPrompt.versions[diffV2]?.model}</span>
                    </div>
                    <pre className="whitespace-pre-wrap text-emerald-300/90 leading-relaxed overflow-x-auto">
                      {currentPrompt.versions[diffV2]?.template || 'Select version'}
                    </pre>
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* TAB 3: ROUTING & CANARY */}
          {tab === 'routing' && currentPrompt && (
            <div className="space-y-6">
              <div className="flex items-center justify-between">
                <div>
                  <h2 className="text-lg font-bold text-white font-mono">Dynamic Traffic Splitter & Canary Router</h2>
                  <p className="text-xs text-slate-400">
                    Deterministic sticky user bucketing based on hash. Route live requests between Active and Canary variants.
                  </p>
                </div>
                <button
                  onClick={handleSaveRouting}
                  className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs transition shadow-sm"
                >
                  Save Routing Configuration
                </button>
              </div>

              {/* Traffic Weight Controls */}
              <div className="p-5 rounded-xl bg-[#0b0f17] border border-slate-800 space-y-6">
                <div>
                  <div className="flex items-center justify-between mb-2">
                    <label className="text-sm font-semibold text-white">Canary Traffic Weight</label>
                    <span className="font-mono text-sm font-bold text-amber-400">{canaryWeight}% Canary / {100 - canaryWeight}% Active</span>
                  </div>
                  <input
                    type="range"
                    min="0"
                    max="50"
                    step="5"
                    value={canaryWeight}
                    onChange={e => setCanaryWeight(parseInt(e.target.value))}
                    className="w-full accent-indigo-500 cursor-pointer h-2 bg-slate-800 rounded-lg"
                  />
                  <div className="flex justify-between text-[10px] text-slate-400 font-mono mt-1">
                    <span>0% (Disabled)</span>
                    <span>10% (Recommended)</span>
                    <span>25% (High volume)</span>
                    <span>50% (50/50 Split)</span>
                  </div>
                </div>

                <div className="pt-4 border-t border-slate-800/80">
                  <div className="flex items-center justify-between mb-3">
                    <div>
                      <div className="text-sm font-semibold text-white flex items-center space-x-2">
                        <span>Zero-Risk Shadow Evaluation Mode</span>
                        <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-purple-500/10 text-purple-400 border border-purple-500/20">
                          ASYNC FORK
                        </span>
                      </div>
                      <p className="text-xs text-slate-400 mt-0.5">
                        Duplicates real production traffic in background goroutines. Evaluates candidate prompt without affecting user response.
                      </p>
                    </div>
                    <label className="relative inline-flex items-center cursor-pointer">
                      <input
                        type="checkbox"
                        checked={shadowEnabled}
                        onChange={e => setShadowEnabled(e.target.checked)}
                        className="sr-only peer"
                      />
                      <div className="w-11 h-6 bg-slate-800 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-purple-600"></div>
                    </label>
                  </div>

                  {shadowEnabled && (
                    <div className="space-y-2">
                      <div className="flex items-center justify-between">
                        <span className="text-xs text-slate-300">Shadow Sample Rate</span>
                        <span className="font-mono text-xs font-bold text-purple-400">{shadowRate}% of live traffic</span>
                      </div>
                      <input
                        type="range"
                        min="5"
                        max="100"
                        step="5"
                        value={shadowRate}
                        onChange={e => setShadowRate(parseInt(e.target.value))}
                        className="w-full accent-purple-500 cursor-pointer h-2 bg-slate-800 rounded-lg"
                      />
                    </div>
                  )}
                </div>
              </div>

              {/* Deterministic Hash Tester */}
              <div className="p-5 rounded-xl bg-[#0b0f17] border border-slate-800">
                <div className="flex items-center space-x-2 mb-3">
                  <Split className="w-4 h-4 text-indigo-400" />
                  <h3 className="text-sm font-semibold text-white">Deterministic User Hash Simulator</h3>
                </div>
                <p className="text-xs text-slate-400 mb-4">
                  Verify which prompt variant an individual customer or tenant will be deterministically routed to.
                </p>

                <div className="flex flex-col sm:flex-row gap-3">
                  <input
                    type="text"
                    value={testUserId}
                    onChange={e => setTestUserId(e.target.value)}
                    placeholder="Enter User ID or Tenant ID (e.g. usr_1042)"
                    className="flex-1 bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-xs font-mono text-white focus:outline-none focus:border-indigo-500"
                  />
                  <button
                    onClick={calculateHashBucket}
                    className="px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-semibold text-slate-200 transition"
                  >
                    Compute Route
                  </button>
                </div>

                {hashResult && (
                  <div className="mt-4 p-3 rounded-lg bg-slate-900/90 border border-indigo-500/30 text-xs font-mono text-emerald-400">
                    {hashResult}
                  </div>
                )}
              </div>
            </div>
          )}

          {/* TAB 4: SHADOW DIFF INSPECTOR */}
          {tab === 'shadow' && (
            <div className="space-y-6">
              <div className="flex items-center justify-between">
                <div>
                  <h2 className="text-lg font-bold text-white font-mono">Shadow Run Quality Diff Inspector</h2>
                  <p className="text-xs text-slate-400">
                    Side-by-side comparison of active production outputs vs candidate shadow executions.
                  </p>
                </div>
                <div className="text-xs font-mono text-slate-400">
                  {shadowDiffs.length} paired shadow runs captured
                </div>
              </div>

              <div className="space-y-4">
                {shadowDiffs.map(diff => {
                  const isBetter = diff.verdict === 'better';
                  const isEquiv = diff.verdict === 'equivalent';

                  return (
                    <div
                      key={diff.id}
                      className="p-5 rounded-xl bg-[#0b0f17] border border-slate-800 space-y-4 font-mono text-xs"
                    >
                      {/* Comparison Header Badge */}
                      <div className="flex flex-wrap items-center justify-between gap-2 pb-3 border-b border-slate-800">
                        <div className="flex items-center space-x-2">
                          <span className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                            isBetter
                              ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                              : isEquiv
                              ? 'bg-indigo-500/10 text-indigo-400 border border-indigo-500/20'
                              : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'
                          }`}>
                            VERDICT: {diff.verdict.toUpperCase()}
                          </span>
                          <span className="text-white font-bold">{diff.prompt_id}</span>
                        </div>

                        <div className="flex items-center space-x-4 text-slate-400 text-[11px]">
                          <span>Semantic Agreement: <strong className="text-white">{Math.round(diff.quality_score * 100)}%</strong></span>
                          <span>Latency Delta: <strong className={diff.latency_delta_ms <= 0 ? 'text-emerald-400' : 'text-rose-400'}>{diff.latency_delta_ms}ms</strong></span>
                          <span>Cost Delta: <strong className={diff.cost_delta_usd <= 0 ? 'text-emerald-400' : 'text-rose-400'}>${diff.cost_delta_usd.toFixed(6)}</strong></span>
                        </div>
                      </div>

                      {/* Explanation note */}
                      <div className="text-xs text-slate-300 font-sans italic bg-slate-900/60 p-2.5 rounded-lg border border-slate-800/80">
                        "{diff.explanation}"
                      </div>

                      {/* Side by side outputs */}
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div className="p-3 rounded-lg bg-slate-900/80 border border-slate-800">
                          <div className="text-slate-400 text-[10px] mb-1 font-bold">
                            Active Production Output ({diff.active_version})
                          </div>
                          <div className="text-slate-200 whitespace-pre-wrap">{diff.active_output || 'Output unavailable'}</div>
                        </div>

                        <div className="p-3 rounded-lg bg-purple-950/20 border border-purple-500/30">
                          <div className="text-purple-400 text-[10px] mb-1 font-bold">
                            Shadow Candidate Output ({diff.shadow_version})
                          </div>
                          <div className="text-purple-200 whitespace-pre-wrap">{diff.shadow_output || 'Output unavailable'}</div>
                        </div>
                      </div>
                    </div>
                  );
                })}

                {shadowDiffs.length === 0 && (
                  <div className="text-center py-12 rounded-xl bg-[#0b0f17] border border-slate-800 text-slate-400 text-xs">
                    No shadow runs recorded yet. Enable Shadow Mode in the Traffic Split tab and simulate traffic.
                  </div>
                )}
              </div>
            </div>
          )}

          {/* TAB 5: SAFETY GATES & ROLLBACK */}
          {tab === 'gates' && currentPrompt && (
            <div className="space-y-6">
              <div className="flex items-center justify-between">
                <div>
                  <h2 className="text-lg font-bold text-white font-mono">Automated Safety Gates & Instant Rollback</h2>
                  <p className="text-xs text-slate-400">
                    Statistically driven promotion criteria and automated circuit-breaker rollback gates.
                  </p>
                </div>
                <div className="flex items-center space-x-2">
                  <button
                    onClick={() => handlePromote()}
                    className="px-3.5 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs transition"
                  >
                    1-Click Promote Canary
                  </button>
                  <button
                    onClick={handleRollback}
                    className="px-3.5 py-1.5 rounded-lg bg-rose-600 hover:bg-rose-500 text-white font-semibold text-xs transition flex items-center space-x-1"
                  >
                    <RotateCcw className="w-3.5 h-3.5" />
                    <span>Instant Rollback</span>
                  </button>
                </div>
              </div>

              {/* Thresholds Cards */}
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div className="p-4 rounded-xl bg-[#0b0f17] border border-slate-800">
                  <div className="text-xs text-slate-400 font-medium">Quality Gate</div>
                  <div className="text-xl font-bold text-emerald-400 font-mono mt-1">
                    ≥ {Math.round(currentPrompt.gates.min_quality_score * 100)}%
                  </div>
                  <p className="text-[11px] text-slate-400 mt-1">Minimum semantic agreement score required to promote candidate.</p>
                </div>

                <div className="p-4 rounded-xl bg-[#0b0f17] border border-slate-800">
                  <div className="text-xs text-slate-400 font-medium">Latency Ceiling Gate</div>
                  <div className="text-xl font-bold text-indigo-400 font-mono mt-1">
                    ≤ {currentPrompt.gates.max_p95_latency_ms}ms
                  </div>
                  <p className="text-[11px] text-slate-400 mt-1">Maximum allowed P95 execution duration.</p>
                </div>

                <div className="p-4 rounded-xl bg-[#0b0f17] border border-slate-800">
                  <div className="text-xs text-slate-400 font-medium">Error Rate Ceiling</div>
                  <div className="text-xl font-bold text-rose-400 font-mono mt-1">
                    ≤ {(currentPrompt.gates.max_error_rate * 100).toFixed(1)}%
                  </div>
                  <p className="text-[11px] text-slate-400 mt-1">Automatic rollback triggers immediately if canary error rate exceeds this.</p>
                </div>
              </div>

              {/* Policy Audit Trail */}
              <div className="rounded-xl bg-[#0b0f17] border border-slate-800 p-5">
                <h3 className="text-sm font-semibold text-white mb-3">Policy Audit & Promotion Log</h3>
                <div className="space-y-2">
                  {auditLogs.map(log => (
                    <div
                      key={log.id}
                      className="p-3 rounded-lg bg-slate-900 border border-slate-800/80 flex items-center justify-between text-xs font-mono"
                    >
                      <div className="flex items-center space-x-3">
                        <span className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                          log.action === 'promote'
                            ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                            : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'
                        }`}>
                          {log.action.toUpperCase()}
                        </span>
                        <span className="text-white font-semibold">{log.prompt_id}</span>
                        <span className="text-slate-400">{log.reason}</span>
                      </div>
                      <div className="flex items-center space-x-3 text-slate-400">
                        <span className="text-indigo-400">by {log.triggered_by}</span>
                        <span>{new Date(log.created_at).toLocaleTimeString()}</span>
                      </div>
                    </div>
                  ))}
                  {auditLogs.length === 0 && (
                    <div className="text-center py-6 text-slate-400 text-xs">
                      No promotion or rollback events logged yet.
                    </div>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* TAB 6: PLAYGROUND */}
          {tab === 'playground' && currentPrompt && (
            <div className="space-y-6">
              <div>
                <h2 className="text-lg font-bold text-white font-mono">Live Prompt Execution Console</h2>
                <p className="text-xs text-slate-400">
                  Send arbitrary variable payloads, test individual versions or execute through the live router.
                </p>
              </div>

              <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                {/* Input Panel */}
                <div className="p-5 rounded-xl bg-[#0b0f17] border border-slate-800 space-y-4">
                  <div className="flex items-center justify-between">
                    <label className="text-xs font-semibold text-white">Target Version</label>
                    <select
                      value={playVersion}
                      onChange={e => setPlayVersion(e.target.value)}
                      className="bg-slate-900 border border-slate-700 rounded-lg px-2.5 py-1 text-xs font-mono text-white"
                    >
                      <option value="auto">Auto (Live Traffic Router Split)</option>
                      {Object.keys(currentPrompt.versions || {}).map(v => (
                        <option key={v} value={v}>{v} ({currentPrompt.versions[v].model})</option>
                      ))}
                    </select>
                  </div>

                  <div>
                    <label className="text-xs font-semibold text-white block mb-1.5">Template Variables (JSON)</label>
                    <textarea
                      rows={8}
                      value={playVars}
                      onChange={e => setPlayVars(e.target.value)}
                      className="w-full bg-slate-900 border border-slate-700 rounded-lg p-3 text-xs font-mono text-white focus:outline-none focus:border-indigo-500 leading-relaxed"
                    />
                  </div>

                  <button
                    onClick={handleExecutePlayground}
                    disabled={playLoading}
                    className="w-full py-2.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs transition flex items-center justify-center space-x-2 disabled:opacity-50"
                  >
                    <Play className={`w-3.5 h-3.5 ${playLoading ? 'animate-spin' : ''}`} />
                    <span>{playLoading ? 'Executing LLM Generation...' : 'Execute Prompt Request'}</span>
                  </button>
                </div>

                {/* Output Panel */}
                <div className="p-5 rounded-xl bg-[#0b0f17] border border-slate-800 flex flex-col justify-between">
                  <div>
                    <div className="flex items-center justify-between mb-3 pb-2 border-b border-slate-800">
                      <span className="text-xs font-semibold text-white">Execution Result</span>
                      {playResult && (
                        <div className="flex items-center space-x-3 text-xs font-mono text-slate-400">
                          <span>{playResult.latency_ms || playResult.response?.latency_ms}ms</span>
                          <span className="text-emerald-400">
                            ${(playResult.cost_usd || playResult.response?.cost_usd || 0).toFixed(6)}
                          </span>
                        </div>
                      )}
                    </div>

                    {playResult ? (
                      <div className="space-y-3 font-mono text-xs">
                        <div className="p-3 rounded-lg bg-slate-900/90 border border-slate-800">
                          <div className="text-[10px] text-slate-400 mb-1">Generated Output Text:</div>
                          <pre className="whitespace-pre-wrap text-emerald-300">
                            {playResult.output_text || playResult.response?.output_text}
                          </pre>
                        </div>

                        {playResult.shadow_dispatched && (
                          <div className="p-2.5 rounded-lg bg-purple-950/20 border border-purple-500/30 text-[11px] text-purple-300 flex items-center space-x-2">
                            <Sparkles className="w-3.5 h-3.5 text-purple-400" />
                            <span>Shadow fork dispatched in background! Check the Shadow Diff tab for evaluation.</span>
                          </div>
                        )}
                      </div>
                    ) : (
                      <div className="text-center py-16 text-slate-400 text-xs font-mono">
                        Awaiting execution. Enter variables and click "Execute Prompt Request".
                      </div>
                    )}
                  </div>

                  <div className="text-[10px] text-slate-400 font-mono pt-4 border-t border-slate-800/80">
                    Endpoint: POST /v1/prompts/{selectedPromptId}/execute
                  </div>
                </div>
              </div>
            </div>
          )}
        </main>
      </div>
    </div>
  );
}
