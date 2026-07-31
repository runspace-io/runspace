'use client';

import {
  forceCenter,
  forceLink,
  forceManyBody,
  forceSimulation,
  type SimulationNodeDatum,
} from 'd3';
import { useMemo } from 'react';
import type { WorkspaceApiClient } from '@/lib/api-client';

type GraphNode = SimulationNodeDatum & {
  id: string;
  label: string;
  resource?: string;
};

type GraphEdge = {
  source: string | GraphNode;
  target: string | GraphNode;
};

export function D3Artifact({
  api,
  workspaceID,
  props,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  props: Record<string, unknown>;
}) {
  const graph = useMemo(() => layoutGraph(props), [props]);
  const requestOpen = (node: GraphNode) => {
    if (!node.resource) return;
    void api.requestUIAction(workspaceID, {
      operation: 'workspace.open_resource',
      resource: node.resource,
      reason: `Open ${node.label} from an interactive visualization.`,
    });
  };
  return (
    <div className="ui-d3-artifact">
      <svg viewBox="0 0 640 320" role="img" aria-label="Interactive dependency graph">
        {graph.edges.map((edge, index) => {
          const source = endpoint(edge.source);
          const target = endpoint(edge.target);
          return (
            <line
              key={`${source.id}:${target.id}:${index}`}
              x1={source.x}
              y1={source.y}
              x2={target.x}
              y2={target.y}
            />
          );
        })}
        {graph.nodes.map((node) => (
          <g
            className={node.resource ? 'is-actionable' : ''}
            key={node.id}
            transform={`translate(${node.x ?? 0} ${node.y ?? 0})`}
            onClick={() => requestOpen(node)}
          >
            <circle r="22" />
            <text y="36" textAnchor="middle">
              {node.label}
            </text>
          </g>
        ))}
      </svg>
      <small>Declarative data · trusted Runspace renderer · no agent JavaScript</small>
    </div>
  );
}

function layoutGraph(props: Record<string, unknown>) {
  const nodes = arrayRecords(props.nodes)
    .slice(0, 100)
    .map((node, index): GraphNode => {
      const resource = text(node.resource);
      return {
        id: text(node.id) || `node-${index}`,
        label: text(node.label) || text(node.id) || `Node ${index + 1}`,
        ...(resource ? { resource } : {}),
      };
    });
  const ids = new Set(nodes.map((node) => node.id));
  const edges: GraphEdge[] = arrayRecords(props.edges)
    .slice(0, 200)
    .map((edge) => ({ source: text(edge.source), target: text(edge.target) }))
    .filter((edge) => ids.has(String(edge.source)) && ids.has(String(edge.target)));
  const simulation = forceSimulation(nodes)
    .force(
      'link',
      forceLink<GraphNode, GraphEdge>(edges)
        .id((node) => node.id)
        .distance(92),
    )
    .force('charge', forceManyBody().strength(-260))
    .force('center', forceCenter(320, 145))
    .stop();
  for (let tick = 0; tick < 180; tick += 1) simulation.tick();
  return { nodes, edges };
}

function endpoint(value: string | GraphNode): GraphNode {
  return typeof value === 'string' ? { id: value, label: value, x: 0, y: 0 } : value;
}

function arrayRecords(value: unknown): Array<Record<string, unknown>> {
  return Array.isArray(value)
    ? value.filter((item): item is Record<string, unknown> =>
        Boolean(item && typeof item === 'object'),
      )
    : [];
}

function text(value: unknown) {
  return typeof value === 'string' ? value : '';
}
