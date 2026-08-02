import {
  ChevronDown,
  ChevronRight,
  FileCode2,
  LibraryBig,
  MessageSquare,
  Package,
  PanelLeft,
  Plus,
  SquareCheckBig,
  Zap,
  X,
} from 'lucide-react';
import type { WorkspaceTreeEntry } from '@/lib/workspace-state';
import type { ApiChannel } from '@/lib/api-client';
import type { ChannelWorkItem } from './use-channel-shared-work';
export function LeftRail({
  open,
  tree,
  expandedDirectories,
  selectedFilePath,
  channels,
  activeChannelID,
  channelWork,
  activeWorkID,
  onClose,
  onRequestCreateChannel,
  onOpenChannel,
  onOpenWork,
  onOpenResourceCenter,
  onSelectFile,
  onToggleDirectory,
}: {
  open: boolean;
  tree: readonly WorkspaceTreeEntry[];
  expandedDirectories: readonly string[];
  selectedFilePath?: string | undefined;
  channels: readonly ApiChannel[];
  activeChannelID?: string | undefined;
  channelWork: readonly ChannelWorkItem[];
  activeWorkID?: string | undefined;
  onClose: () => void;
  onRequestCreateChannel: (parentID?: string) => void;
  onOpenChannel: (channel: ApiChannel) => void;
  onOpenWork: (item: ChannelWorkItem) => void;
  onOpenResourceCenter: () => void;
  onSelectFile: (path: string) => void;
  onToggleDirectory: (path: string) => void;
}) {
  return (
    <aside className={`left-rail ${open ? 'is-open' : ''}`}>
      <div className="rail-heading">
        <span>Workspace</span>
        <button className="icon-button mobile-only" aria-label="Close navigation" onClick={onClose}>
          <X size={16} />
        </button>
      </div>
      <button className="resource-center-nav" onClick={onOpenResourceCenter}>
        <LibraryBig size={15} />
        <span>
          <strong>Resource Center</strong>
          <small>Shared workspace knowledge</small>
        </span>
      </button>
      <div className="rail-section-header">
        <span>Channels</span>
        <button
          className="tiny-button"
          aria-label="Add channel"
          onClick={() => onRequestCreateChannel()}
        >
          <Plus size={14} />
        </button>
      </div>
      <div className="file-tree channel-tree" role="tree" aria-label="Workspace channels">
        {channels.map((channel) => (
          <div className="channel-tree-node" key={channel.id}>
            <div className="channel-tree-row">
              <button
                className={`tree-item ${activeChannelID === channel.id ? 'active' : ''}`}
                aria-label={`Open ${channel.name}`}
                aria-current={activeChannelID === channel.id ? 'page' : undefined}
                onClick={() => onOpenChannel(channel)}
              >
                <span>{channel.parent_id ? '↳' : '#'} </span>
                <span>{channel.name}</span>
              </button>
              <button
                className="tiny-button"
                aria-label={`Add subchannel to ${channel.name}`}
                onClick={() => onRequestCreateChannel(channel.id)}
              >
                <Plus size={12} />
              </button>
            </div>
            {activeChannelID === channel.id && channelWork.length > 0 && (
              <div
                className="channel-chat-children"
                role="group"
                aria-label={`${channel.name} shared work`}
              >
                <span className="channel-work-heading">Shared work</span>
                {channelWork.map((item) => (
                  <button
                    key={item.node.id}
                    type="button"
                    className={`tree-item ${activeWorkID === item.node.id ? 'active' : ''}`}
                    aria-label={`Open shared ${item.node.kind} ${item.node.title}`}
                    onClick={() => onOpenWork(item)}
                  >
                    <WorkIcon item={item} />
                    <span>{item.node.title}</span>
                    <small>{item.node.kind}</small>
                  </button>
                ))}
              </div>
            )}
          </div>
        ))}
        {channels.length === 0 && <span className="empty-state">No channels yet</span>}
      </div>
      <ResourceFileTree
        tree={tree}
        expandedDirectories={expandedDirectories}
        selectedFilePath={selectedFilePath}
        onSelectFile={onSelectFile}
        onToggleDirectory={onToggleDirectory}
      />
    </aside>
  );
}

function ResourceFileTree({
  tree,
  expandedDirectories,
  selectedFilePath,
  onSelectFile,
  onToggleDirectory,
}: {
  tree: readonly WorkspaceTreeEntry[];
  expandedDirectories: readonly string[];
  selectedFilePath?: string | undefined;
  onSelectFile: (path: string) => void;
  onToggleDirectory: (path: string) => void;
}) {
  if (tree.length === 0) return null;
  return (
    <>
      <div className="rail-section-header">
        <span>Files</span>
        <PanelLeft size={14} />
      </div>
      <div className="file-tree resource-tree" role="tree" aria-label="Resource files">
        {tree.map((entry) => (
          <button
            className={`tree-item ${selectedFilePath === entry.path ? 'active' : ''}`}
            role="treeitem"
            style={{ paddingLeft: `${12 + (entry.path.split('/').length - 1) * 16}px` }}
            key={entry.path}
            aria-expanded={
              entry.kind === 'directory' ? expandedDirectories.includes(entry.path) : undefined
            }
            aria-selected={selectedFilePath === entry.path}
            onClick={() =>
              entry.kind === 'file' ? onSelectFile(entry.path) : onToggleDirectory(entry.path)
            }
          >
            {entry.kind === 'directory' ? (
              expandedDirectories.includes(entry.path) ? (
                <ChevronDown size={13} />
              ) : (
                <ChevronRight size={13} />
              )
            ) : (
              <FileCode2 size={14} />
            )}
            <span>{entry.path.split('/').at(-1)}</span>
          </button>
        ))}
      </div>
    </>
  );
}

function WorkIcon({ item }: { item: ChannelWorkItem }) {
  if (item.chat) return <MessageSquare size={12} />;
  if (item.node.kind === 'artifact') return <Package size={12} />;
  if (item.node.kind === 'action') return <Zap size={12} />;
  return <SquareCheckBig size={12} />;
}
