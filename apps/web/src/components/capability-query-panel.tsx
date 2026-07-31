'use client';

import { Search } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import type { ApiGraphNode, WorkspaceApiClient } from '@/lib/api-client';

type Match = {
  title: string;
  summary?: string;
  reference?: string;
  metadata?: Record<string, unknown>;
};

export function CapabilityQueryPanel({
  api,
  workspaceID,
  node,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  node: ApiGraphNode;
}) {
  const capabilities = useMemo(() => advertisedCapabilities(node), [node]);
  const [capability, setCapability] = useState(capabilities[0]?.id ?? '');
  const [query, setQuery] = useState('');
  const [matches, setMatches] = useState<Match[]>([]);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [availability, setAvailability] = useState<{
    status: 'checking' | 'available' | 'unavailable';
    reason?: string;
  }>({ status: 'checking' });
  useEffect(() => {
    if (!capabilities.length) return;
    let active = true;
    void api
      .getGraphResourceAvailability(workspaceID, node.id)
      .then((result) => active && setAvailability(result))
      .catch((reason: unknown) => {
        if (active) {
          setAvailability({
            status: 'unavailable',
            reason: reason instanceof Error ? reason.message : 'Owner host is unavailable.',
          });
        }
      });
    return () => {
      active = false;
    };
  }, [api, capabilities.length, node.id, workspaceID]);
  if (!capabilities.length) return null;
  const run = async () => {
    setBusy(true);
    setError('');
    try {
      const result = await api.queryGraphResource(workspaceID, node.id, {
        capability,
        query,
        limit: 20,
      });
      setMatches(result.matches ?? []);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Resource query failed.');
    } finally {
      setBusy(false);
    }
  };
  return (
    <section className="capability-query-panel">
      <header>
        <div>
          <p className="eyebrow">Owner-hosted query</p>
          <h3>Explore this Resource</h3>
        </div>
        <AvailabilityLabel status={availability.status} />
      </header>
      <div className="capability-query-controls">
        <select value={capability} onChange={(event) => setCapability(event.target.value)}>
          {capabilities.map((item) => (
            <option value={item.id} key={item.id}>
              {item.label}
            </option>
          ))}
        </select>
        <label>
          <Search size={14} />
          <input
            value={query}
            maxLength={240}
            placeholder="Search the connected system"
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={(event) => event.key === 'Enter' && void run()}
          />
        </label>
        <button
          className="primary-button"
          disabled={busy || !capability || availability.status !== 'available'}
          onClick={() => void run()}
        >
          {queryButtonLabel(busy)}
        </button>
      </div>
      <QueryFeedback error={availability.reason || error} />
      <CapabilityResults matches={matches} busy={busy} error={error} />
    </section>
  );
}

function AvailabilityLabel({ status }: { status: 'checking' | 'available' | 'unavailable' }) {
  return <span data-status={status}>{status === 'checking' ? 'checking…' : status}</span>;
}

function QueryFeedback({ error }: { error: string | undefined }) {
  return error ? <p className="agent-task-error">{error}</p> : null;
}

function CapabilityResults({
  matches,
  busy,
  error,
}: {
  matches: Match[];
  busy: boolean;
  error: string;
}) {
  return (
    <div className="capability-results">
      {matches.map((match) => (
        <article key={`${match.reference ?? ''}:${match.title}`}>
          <strong>{match.title}</strong>
          {match.summary ? <p>{match.summary}</p> : null}
          {match.reference ? <code>{match.reference}</code> : null}
        </article>
      ))}
      {!busy && !error && matches.length === 0 ? (
        <p>Run a query to receive bounded, structured results from the owner’s machine.</p>
      ) : null}
    </div>
  );
}

function queryButtonLabel(busy: boolean) {
  return busy ? 'Querying…' : 'Query';
}

function advertisedCapabilities(node: ApiGraphNode): Array<{ id: string; label: string }> {
  const raw = node.metadata?.capabilities;
  if (!Array.isArray(raw)) return [];
  return raw.flatMap((item) => {
    if (!item || typeof item !== 'object') return [];
    const candidate = item as Record<string, unknown>;
    return typeof candidate.id === 'string' && typeof candidate.label === 'string'
      ? [{ id: candidate.id, label: candidate.label }]
      : [];
  });
}
