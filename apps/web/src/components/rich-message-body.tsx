'use client';

import { useEffect, useState } from 'react';
import type { ApiGraphNode, ApiUIDocument, WorkspaceApiClient } from '@/lib/api-client';
import { RichMarkdown, UIArtifactRenderer } from './ui-artifact-renderer';

const liveReference = /\[\[(ui|resource|file|diff):([^\]]+)\]\]/g;

export function RichMessageBody({
  api,
  workspaceID,
  body,
  onOpenNode,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  body: string;
  onOpenNode: (node: ApiGraphNode) => void;
}) {
  const parts = splitMessage(body);
  return (
    <div className="rich-message-body">
      {parts.map((part, index) =>
        part.kind === 'ui' && part.value ? (
          <ArtifactReference
            api={api}
            workspaceID={workspaceID}
            nodeID={part.value}
            key={`${part.value}:${index}`}
          />
        ) : part.kind && part.value ? (
          <WorkspaceReference
            api={api}
            workspaceID={workspaceID}
            kind={part.kind}
            value={part.value}
            onOpenNode={onOpenNode}
            key={`${part.kind}:${index}`}
          />
        ) : (
          <RichMarkdown content={part.text} key={`text:${index}`} />
        ),
      )}
    </div>
  );
}

function ArtifactReference({
  api,
  workspaceID,
  nodeID,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  nodeID: string;
}) {
  const [document, setDocument] = useState<ApiUIDocument>();
  const [error, setError] = useState('');
  useEffect(() => {
    let active = true;
    void api
      .getGraphContext(workspaceID, nodeID)
      .then((context) => {
        const value = context.node.metadata?.ui_document;
        if (active && isUIDocument(value)) setDocument(value);
        else if (active) setError('Interactive artifact is invalid or unavailable.');
      })
      .catch(() => active && setError('Interactive artifact could not be loaded.'));
    return () => {
      active = false;
    };
  }, [api, nodeID, workspaceID]);
  if (error) return <p className="ui-artifact-error">{error}</p>;
  if (!document) return <p className="graph-loading">Loading interactive artifact…</p>;
  return <UIArtifactRenderer api={api} workspaceID={workspaceID} document={document} />;
}

function WorkspaceReference({
  api,
  workspaceID,
  kind,
  value,
  onOpenNode,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  kind: string;
  value: string;
  onOpenNode: (node: ApiGraphNode) => void;
}) {
  const [node, setNode] = useState<ApiGraphNode>();
  const [unavailable, setUnavailable] = useState(false);
  useEffect(() => {
    if (kind !== 'resource') return;
    let active = true;
    void api
      .getGraphContext(workspaceID, value)
      .then((context) => active && setNode(context.node))
      .catch(() => active && setUnavailable(true));
    return () => {
      active = false;
    };
  }, [api, kind, value, workspaceID]);
  const open = () => {
    if (kind !== 'resource') return;
    if (node) onOpenNode(node);
  };
  const label =
    kind === 'resource'
      ? (node?.title ?? (unavailable ? 'Unavailable Resource' : 'Loading Resource…'))
      : value;
  return (
    <button
      className="message-live-reference"
      data-kind={kind}
      disabled={kind === 'resource' && !node}
      title={kind === 'resource' ? value : undefined}
      onClick={open}
    >
      <span>{kind}</span>
      <strong>{label}</strong>
    </button>
  );
}

function splitMessage(body: string): Array<{ text: string; kind?: string; value?: string }> {
  const result: Array<{ text: string; kind?: string; value?: string }> = [];
  let cursor = 0;
  for (const match of body.matchAll(liveReference)) {
    const index = match.index ?? 0;
    if (index > cursor) result.push({ text: body.slice(cursor, index) });
    if (match[1] && match[2]) result.push({ text: '', kind: match[1], value: match[2] });
    cursor = index + match[0].length;
  }
  if (cursor < body.length) result.push({ text: body.slice(cursor) });
  return result.length ? result : [{ text: body }];
}

function isUIDocument(value: unknown): value is ApiUIDocument {
  if (!value || typeof value !== 'object') return false;
  const candidate = value as Record<string, unknown>;
  return (
    candidate.version === 'runspace.ui/v1' &&
    typeof candidate.title === 'string' &&
    Boolean(candidate.layout && typeof candidate.layout === 'object')
  );
}
