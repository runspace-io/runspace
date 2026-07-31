'use client';

import {
  Box,
  Database,
  FileText,
  LibraryBig,
  MessageSquare,
  Plus,
  Search,
  SquareCheckBig,
  X,
  Zap,
  type LucideIcon,
} from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import type { ApiGraphKind, ApiGraphNode, WorkspaceApiClient } from '@/lib/api-client';
import { displayName } from '@/lib/message-identity';
import { CapabilityConnectionDialog } from './capability-connection-dialog';

const filters: { label: string; kind?: ApiGraphKind }[] = [
  { label: 'Everything' },
  { label: 'Resources', kind: 'resource' },
  { label: 'Artifacts', kind: 'artifact' },
  { label: 'Tasks', kind: 'task' },
  { label: 'Actions', kind: 'action' },
  { label: 'Discussions', kind: 'discussion' },
];

export function ResourceCenter({
  api,
  workspaceID,
  onOpen,
  onClose,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  onOpen: (node: ApiGraphNode) => void;
  onClose: () => void;
}) {
  const [nodes, setNodes] = useState<ApiGraphNode[]>([]);
  const [query, setQuery] = useState('');
  const [kind, setKind] = useState<ApiGraphKind>();
  const [loading, setLoading] = useState(true);
  const [connectOpen, setConnectOpen] = useState(false);
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    let active = true;
    setLoading(true);
    void api
      .listGraphNodes(workspaceID, { limit: 200 })
      .then((items) => active && setNodes(items))
      .catch(() => active && setNodes([]))
      .finally(() => active && setLoading(false));
    return () => {
      active = false;
    };
  }, [api, revision, workspaceID]);
  const visible = useMemo(
    () => nodes.filter((node) => matchesResource(node, kind, query)),
    [kind, nodes, query],
  );
  return (
    <section className="resource-center" aria-labelledby="resource-center-title">
      <header className="resource-center-header">
        <span className="resource-center-mark">
          <LibraryBig size={18} />
        </span>
        <div>
          <p className="eyebrow">Workspace knowledge</p>
          <h2 id="resource-center-title">Resource Center</h2>
          <p>Shared resources that members and permitted agents can discover and use.</p>
        </div>
        <div className="resource-center-actions">
          <button className="secondary-button" onClick={() => setConnectOpen(true)}>
            <Plus size={14} />
            Connect
          </button>
          <button className="icon-button" onClick={onClose} aria-label="Close Resource Center">
            <X size={16} />
          </button>
        </div>
      </header>
      <div className="resource-center-controls">
        <label className="resource-center-search">
          <Search size={14} />
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search shared resources"
          />
        </label>
        <nav aria-label="Resource types">
          {filters.map((filter) => (
            <button
              key={filter.label}
              className={kind === filter.kind ? 'is-active' : ''}
              onClick={() => setKind(filter.kind)}
            >
              {filter.label}
              <span>{countKind(nodes, filter.kind)}</span>
            </button>
          ))}
        </nav>
      </div>
      <div className="resource-center-grid">
        {visible.map((node) => (
          <ResourceCard key={node.id} node={node} onOpen={onOpen} />
        ))}
        {!loading && visible.length === 0 ? (
          <div className="resource-center-empty">
            <Box size={20} />
            <strong>No shared resources match</strong>
            <span>Connect or publish something into this workspace.</span>
          </div>
        ) : null}
        {loading ? <p className="graph-loading">Reading workspace graph…</p> : null}
      </div>
      {connectOpen ? (
        <CapabilityConnectionDialog
          api={api}
          userID={api.actorID}
          workspaceID={workspaceID}
          onClose={() => setConnectOpen(false)}
          onConnected={() => {
            setConnectOpen(false);
            setRevision((value) => value + 1);
          }}
        />
      ) : null}
    </section>
  );
}

function ResourceCard({
  node,
  onOpen,
}: {
  node: ApiGraphNode;
  onOpen: (node: ApiGraphNode) => void;
}) {
  const Icon = kindIcon(node.kind);
  return (
    <button className="resource-card" onClick={() => onOpen(node)}>
      <span className="resource-card-icon" data-kind={node.kind}>
        <Icon size={16} />
      </span>
      <span className="resource-card-copy">
        <span>
          <strong>{node.title}</strong>
          <small>{node.kind}</small>
        </span>
        <p>{node.summary || node.type.replaceAll('_', ' ')}</p>
        <small>Shared by {displayName(node.owner_id)}</small>
      </span>
    </button>
  );
}

function matchesResource(node: ApiGraphNode, kind: ApiGraphKind | undefined, query: string) {
  if (kind && node.kind !== kind) return false;
  const needle = query.trim().toLocaleLowerCase();
  if (!needle) return true;
  return `${node.title} ${node.summary ?? ''} ${node.type} ${node.owner_id}`
    .toLocaleLowerCase()
    .includes(needle);
}

function countKind(nodes: readonly ApiGraphNode[], kind: ApiGraphKind | undefined) {
  return kind ? nodes.filter((node) => node.kind === kind).length : nodes.length;
}

function kindIcon(kind: ApiGraphKind): LucideIcon {
  if (kind === 'artifact') return FileText;
  if (kind === 'task') return SquareCheckBig;
  if (kind === 'action') return Zap;
  if (kind === 'discussion') return MessageSquare;
  if (kind === 'resource') return Database;
  return Box;
}
