'use client';

import { CheckCircle2, Code2, FileDiff, FlaskConical, ShieldCheck } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import type { ReactNode } from 'react';
import rehypeSanitize from 'rehype-sanitize';
import remarkGfm from 'remark-gfm';
import type { ApiUIComponentNode, ApiUIDocument, WorkspaceApiClient } from '@/lib/api-client';
import { D3Artifact } from './d3-artifact';
import { rehypeMentions } from './markdown-mentions';

export function UIArtifactRenderer({
  api,
  workspaceID,
  document,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  document: ApiUIDocument;
}) {
  if (document.version !== 'runspace.ui/v1') return null;
  return (
    <section className="ui-artifact" aria-label={document.title}>
      <header>
        <span>Interactive artifact</span>
        <strong>{document.title}</strong>
      </header>
      <UINode api={api} workspaceID={workspaceID} node={document.layout} />
    </section>
  );
}

function UINode({
  api,
  workspaceID,
  node,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  node: ApiUIComponentNode;
}) {
  if (node.type === 'Stack' || node.type === 'Grid') {
    return (
      <div className={`ui-layout ui-layout--${node.type.toLowerCase()}`}>
        {(node.children ?? []).slice(0, 50).map((child, index) => (
          <UINode api={api} workspaceID={workspaceID} node={child} key={`${child.type}:${index}`} />
        ))}
      </div>
    );
  }
  return <UIComponent api={api} workspaceID={workspaceID} node={node} />;
}

function UIComponent({
  api,
  workspaceID,
  node,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  node: ApiUIComponentNode;
}) {
  const props = node.props ?? {};
  if (node.type === 'ApprovalRequest')
    return <ApprovalRequest api={api} workspaceID={workspaceID} props={props} />;
  if (node.type === 'D3Artifact')
    return <D3Artifact api={api} workspaceID={workspaceID} props={props} />;
  return <StaticUIComponent type={node.type} props={props} />;
}

function StaticUIComponent({ type, props }: { type: string; props: Record<string, unknown> }) {
  switch (type) {
    case 'Markdown':
      return <RichMarkdown content={stringProp(props, 'content')} />;
    case 'MetricGroup':
      return <MetricGroup props={props} />;
    case 'TaskCard':
      return <TaskCard props={props} />;
    case 'CodeReference':
      return <ReferenceCard icon={<Code2 size={15} />} props={props} />;
    case 'DiffViewer':
      return <ReferenceCard icon={<FileDiff size={15} />} props={props} />;
    case 'TestReport':
      return <TestReport props={props} />;
    case 'Timeline':
      return <ArtifactTimeline props={props} />;
    case 'DataTable':
      return <DataTable props={props} />;
    default:
      return null;
  }
}

export function RichMarkdown({ content }: { content: string }) {
  return (
    <div className="rich-markdown">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeSanitize, rehypeMentions]}
        components={{ table: MarkdownTable }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}

function MarkdownTable({ children }: { children?: ReactNode }) {
  return (
    <div className="rich-table-wrap">
      <table>{children}</table>
    </div>
  );
}

function MetricGroup({ props }: { props: Record<string, unknown> }) {
  return (
    <div className="ui-metrics">
      {arrayRecords(props.items)
        .slice(0, 12)
        .map((item, index) => (
          <article key={`${stringProp(item, 'label')}:${index}`}>
            <strong>{scalar(item.value)}</strong>
            <span>{stringProp(item, 'label')}</span>
          </article>
        ))}
    </div>
  );
}

function TaskCard({ props }: { props: Record<string, unknown> }) {
  return (
    <article className="ui-task-card">
      <CheckCircle2 size={16} />
      <div>
        <strong>{stringProp(props, 'title')}</strong>
        <p>{stringProp(props, 'summary')}</p>
      </div>
      <span>{stringProp(props, 'status')}</span>
    </article>
  );
}

function ReferenceCard({ icon, props }: { icon: ReactNode; props: Record<string, unknown> }) {
  const reference = stringProp(props, 'resource');
  return (
    <article className="ui-reference">
      {icon}
      <div>
        <strong>{stringProp(props, 'path') || reference}</strong>
        <code>{reference}</code>
      </div>
    </article>
  );
}

function TestReport({ props }: { props: Record<string, unknown> }) {
  return (
    <article className="ui-test-report">
      <FlaskConical size={17} />
      <div>
        <strong>{stringProp(props, 'title') || 'Test report'}</strong>
        <span>
          {scalar(props.passed)} passed · {scalar(props.failed)} failed
        </span>
      </div>
      <code>{stringProp(props, 'resource')}</code>
    </article>
  );
}

function ArtifactTimeline({ props }: { props: Record<string, unknown> }) {
  return (
    <ol className="ui-timeline">
      {arrayRecords(props.items).map((item, index) => (
        <li key={`${stringProp(item, 'label')}:${index}`}>
          <span />
          <div>
            <strong>{stringProp(item, 'label')}</strong>
            <p>{stringProp(item, 'description')}</p>
          </div>
          <time>{stringProp(item, 'time')}</time>
        </li>
      ))}
    </ol>
  );
}

function DataTable({ props }: { props: Record<string, unknown> }) {
  const columns = arrayRecords(props.columns).slice(0, 12);
  const rows = arrayRecords(props.rows).slice(0, 100);
  return (
    <div className="ui-table-wrap">
      <table>
        <thead>
          <tr>
            {columns.map((column) => (
              <th key={stringProp(column, 'key')}>{stringProp(column, 'label')}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => (
            <tr key={index}>
              {columns.map((column) => (
                <td key={stringProp(column, 'key')}>{scalar(row[stringProp(column, 'key')])}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ApprovalRequest({
  api,
  workspaceID,
  props,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  props: Record<string, unknown>;
}) {
  return (
    <article className="ui-approval">
      <ShieldCheck size={18} />
      <div>
        <strong>{stringProp(props, 'title')}</strong>
        <p>{stringProp(props, 'reason')}</p>
      </div>
      <button
        onClick={() =>
          void api.requestUIAction(workspaceID, {
            operation: stringProp(props, 'operation'),
            resource: stringProp(props, 'resource'),
            reason: stringProp(props, 'reason'),
          })
        }
      >
        Request approval
      </button>
    </article>
  );
}

function arrayRecords(value: unknown): Array<Record<string, unknown>> {
  return Array.isArray(value)
    ? value.filter((item): item is Record<string, unknown> =>
        Boolean(item && typeof item === 'object'),
      )
    : [];
}

function stringProp(props: Record<string, unknown>, key: string) {
  return typeof props[key] === 'string' ? props[key] : '';
}

function scalar(value: unknown) {
  return typeof value === 'string' || typeof value === 'number' ? String(value) : '—';
}
