'use client';

import { ArrowDownLeft, ArrowUpRight, Box, ExternalLink, X } from 'lucide-react';
import { useEffect, useState, type ReactNode } from 'react';
import type {
  ApiGraphContext,
  ApiGraphNode,
  ApiUIDocument,
  WorkspaceApiClient,
} from '@/lib/api-client';
import { CapabilityQueryPanel } from './capability-query-panel';
import { UIArtifactRenderer } from './ui-artifact-renderer';

export function GraphNodeSurface({
  api,
  workspaceID,
  node,
  onClose,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  node: ApiGraphNode;
  onClose: () => void;
}) {
  const [context, setContext] = useState<ApiGraphContext>();
  const [error, setError] = useState('');
  useEffect(() => {
    let active = true;
    void api
      .getGraphContext(workspaceID, node.id)
      .then((value) => active && setContext(value))
      .catch((reason: unknown) => active && setError(errorMessage(reason)));
    return () => {
      active = false;
    };
  }, [api, node.id, workspaceID]);
  const current = context?.node ?? node;
  return (
    <section className="graph-node-surface" aria-labelledby="graph-node-title">
      <header className="graph-node-header">
        <div className="graph-node-mark" data-kind={current.kind}>
          <Box size={15} />
        </div>
        <div>
          <p className="eyebrow">
            {current.kind} / {current.type.replaceAll('_', ' ')}
          </p>
          <h2 id="graph-node-title">{current.title}</h2>
        </div>
        <button className="icon-button" onClick={onClose} aria-label="Back to channel">
          <X size={16} />
        </button>
      </header>
      <GraphNodeBody
        api={api}
        workspaceID={workspaceID}
        current={current}
        context={context}
        error={error}
      />
    </section>
  );
}

function GraphNodeBody({
  api,
  workspaceID,
  current,
  context,
  error,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  current: ApiGraphNode;
  context: ApiGraphContext | undefined;
  error: string;
}) {
  const externalRef = safeExternalRef(current.external_ref);
  const summary = current.summary || 'Shared workspace context with no additional summary.';
  const document = uiDocument(current);
  return (
    <div className="graph-node-body">
      <p className="graph-node-summary">{summary}</p>
      {externalRef && (
        <a className="graph-external-ref" href={externalRef}>
          Open source
          <ExternalLink size={13} />
        </a>
      )}
      {document ? (
        <UIArtifactRenderer api={api} workspaceID={workspaceID} document={document} />
      ) : null}
      <CapabilityQueryPanel api={api} workspaceID={workspaceID} node={current} />
      <RelationshipList
        title="Points to"
        icon={<ArrowUpRight size={13} />}
        edges={context?.outgoing ?? []}
        side="to_id"
      />
      <RelationshipList
        title="Referenced by"
        icon={<ArrowDownLeft size={13} />}
        edges={context?.incoming ?? []}
        side="from_id"
      />
      <GraphLoadState context={context} error={error} />
    </div>
  );
}

function uiDocument(node: ApiGraphNode): ApiUIDocument | undefined {
  const value = node.metadata?.ui_document;
  if (!value || typeof value !== 'object') return undefined;
  const candidate = value as Record<string, unknown>;
  if (
    candidate.version !== 'runspace.ui/v1' ||
    typeof candidate.title !== 'string' ||
    !candidate.layout ||
    typeof candidate.layout !== 'object'
  )
    return undefined;
  return value as ApiUIDocument;
}

function GraphLoadState({
  context,
  error,
}: {
  context: ApiGraphContext | undefined;
  error: string;
}) {
  if (error) return <p className="agent-task-error">{error}</p>;
  if (!context) return <p className="graph-loading">Reading relationships…</p>;
  return null;
}

function RelationshipList({
  title,
  icon,
  edges,
  side,
}: {
  title: string;
  icon: ReactNode;
  edges: ApiGraphContext['incoming'];
  side: 'from_id' | 'to_id';
}) {
  if (edges.length === 0) return null;
  return (
    <section className="graph-relations">
      <h3>{title}</h3>
      {edges.map((edge) => (
        <div className="graph-relation" key={edge.id}>
          {icon}
          <span>{edge.relation.replaceAll('_', ' ')}</span>
          <code>{edge[side]}</code>
        </div>
      ))}
    </section>
  );
}

function errorMessage(reason: unknown) {
  return reason instanceof Error ? reason.message : 'Could not load graph context.';
}

function safeExternalRef(value: string | undefined) {
  if (!value) return '';
  try {
    const url = new URL(value);
    return url.protocol === 'https:' || url.protocol === 'http:' ? value : '';
  } catch {
    return '';
  }
}
