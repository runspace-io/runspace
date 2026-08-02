'use client';

import { useState } from 'react';
import type { useWorkspaceController } from './use-workspace-controller';
import { WorkspaceMainColumn, type ToolPanel } from './workspace-main-column';
import { ResizableWorkspacePanels } from './resizable-workspace-panels';
import type { ConnectionDialogMode } from './channel-connection-dialog';
import { useTerminalSessions } from './use-terminal-sessions';
import { channelAgentID } from './run-actions';
import type { ApiGraphNode } from '@/lib/api-client';
import { AgentTaskSurface } from './agent-task-surface';
import type { AgentChatSelection } from './channel-agent-chats';
import { ChannelHeaderPopovers } from './channel-header-popovers';
import { useChannelSharedWork } from './use-channel-shared-work';
import { WorkspaceShellOverlays } from './workspace-shell-overlays';
import { WorkspacePageTopbar } from './workspace-page-topbar';
import { graphNodeSurface, resourceCenterSurface } from './resource-center-surface';
import { workspacePageNavigation } from './workspace-page-navigation';

type Controller = ReturnType<typeof useWorkspaceController>;
export function WorkspacePageShell({
  controller,
  onSignOut,
}: {
  controller: Controller;
  onSignOut: () => void;
}) {
  const [toolPanel, setToolPanel] = useState<ToolPanel>();
  const [channelDialogOpen, setChannelDialogOpen] = useState(false);
  const [channelSettingsOpen, setChannelSettingsOpen] = useState(false);
  const [membersOpen, setMembersOpen] = useState(false);
  const [publishOpen, setPublishOpen] = useState(false);
  const [connectionMode, setConnectionMode] = useState<ConnectionDialogMode>();
  const [channelParentID, setChannelParentID] = useState('');
  const [terminalRepositoryID, setTerminalRepositoryID] = useState<string>();
  const [agentChat, setAgentChat] = useState<AgentChatSelection>();
  const [graphNode, setGraphNode] = useState<ApiGraphNode>();
  const [resourceCenterOpen, setResourceCenterOpen] = useState(false);
  const [chatRevision, setChatRevision] = useState(0);
  const model = pageModel(controller, channelParentID);
  const terminals = useTerminalSessions(controller.api, model.workspaceID);
  const channelWork = useChannelSharedWork(
    controller.api,
    model.workspaceID,
    controller.threadID,
    chatRevision,
  );
  const workSurface = selectedWorkSurface({
    controller,
    model,
    chat: agentChat,
    node: graphNode,
    setChat: setAgentChat,
    setNode: setGraphNode,
    onChange: () => setChatRevision((value) => value + 1),
    resourceCenterOpen,
    closeResourceCenter: () => setResourceCenterOpen(false),
  });
  return (
    <main className="workspace-app">
      <WorkspacePageTopbar
        controller={controller}
        workspaceName={model.workspaceName}
        onOpenMembers={() => setMembersOpen(true)}
        onSignOut={onSignOut}
      />
      <ResizableWorkspacePanels
        navigation={workspacePageNavigation({
          controller,
          channelWork,
          agentChat,
          graphNode,
          setAgentChat,
          setGraphNode,
          setChannelParentID,
          setChannelDialogOpen,
          setResourceCenterOpen,
          setToolPanel,
        })}
        main={
          <WorkspaceMainColumn
            api={controller.api}
            agentChat={workSurface}
            workspaceID={model.workspaceID}
            workspaceName={model.workspaceName}
            channelName={model.channelName}
            repositoryID={controller.selectedRepositoryID}
            chatReady={Boolean(controller.threadID)}
            timeline={controller.timeline}
            draft={controller.draft}
            file={controller.selected}
            toolPanel={toolPanel}
            onDraftChange={controller.setDraft}
            onSend={controller.send}
            runAvailable={controller.agentAvailable && Boolean(controller.selectedRepositoryID)}
            onRunAgent={() => void controller.runAgent()}
            onOpenGraphNode={setGraphNode}
            onOpenChannelSettings={() => setChannelSettingsOpen(true)}
            channelPopovers={
              <ChannelHeaderPopovers
                controller={controller}
                channel={model.activeChannel}
                chatRevision={chatRevision}
                onRequestConnection={setConnectionMode}
                onOpenAgentTask={() => setAgentChat(newAgentChat(controller, model))}
                onOpenAgentChat={setAgentChat}
                onChatShared={(chat) => {
                  setAgentChat(chat);
                  setChatRevision((value) => value + 1);
                }}
                onOpenRepositoryTool={(repositoryID, tool) =>
                  openRepositoryTool(
                    controller,
                    setToolPanel,
                    setTerminalRepositoryID,
                    repositoryID,
                    tool,
                  )
                }
              />
            }
            publishAvailable={model.publishAvailable}
            onOpenPublish={() => setPublishOpen(true)}
            terminalState={terminals}
            terminalRepositories={controller.repositoryOptions}
            onTerminalOpen={(repository) => setTerminalRepositoryID(repository?.id)}
          />
        }
      />
      <WorkspaceShellOverlays
        controller={controller}
        channelDialogOpen={channelDialogOpen}
        channelSettingsOpen={channelSettingsOpen}
        membersOpen={membersOpen}
        channelParentID={channelParentID}
        parentName={model.parentName}
        connectionMode={connectionMode}
        publishOpen={publishOpen}
        terminalRepositoryID={terminalRepositoryID}
        terminals={terminals}
        setChannelDialogOpen={setChannelDialogOpen}
        setChannelSettingsOpen={setChannelSettingsOpen}
        setMembersOpen={setMembersOpen}
        setConnectionMode={setConnectionMode}
        setPublishOpen={setPublishOpen}
        setTerminalRepositoryID={setTerminalRepositoryID}
        setToolPanel={setToolPanel}
      />
    </main>
  );
}

function selectedWorkSurface({
  controller,
  model,
  chat,
  node,
  setChat,
  setNode,
  onChange,
  resourceCenterOpen,
  closeResourceCenter,
}: {
  controller: Controller;
  model: ReturnType<typeof pageModel>;
  chat: AgentChatSelection | undefined;
  node: ApiGraphNode | undefined;
  setChat: (chat: AgentChatSelection | undefined) => void;
  setNode: (node: ApiGraphNode | undefined) => void;
  onChange: () => void;
  resourceCenterOpen: boolean;
  closeResourceCenter: () => void;
}) {
  return (
    graphNodeSurface(controller.api, model.workspaceID, node, () => setNode(undefined)) ??
    resourceCenterSurface(
      controller.api,
      model.workspaceID,
      resourceCenterOpen,
      setNode,
      closeResourceCenter,
    ) ??
    selectedAgentChatSurface(controller, model, chat, setChat, onChange)
  );
}

function openRepositoryTool(
  controller: Controller,
  setToolPanel: (tool: ToolPanel) => void,
  setTerminalRepositoryID: (repositoryID: string) => void,
  repositoryID: string,
  tool: Exclude<ToolPanel, undefined>,
) {
  controller.setSelectedRepositoryID(repositoryID);
  if (tool === 'terminal') {
    setTerminalRepositoryID(repositoryID);
    return;
  }
  setToolPanel(tool);
}

function pageModel(controller: Controller, parentID: string) {
  const activeChannel = controller.channels.find(
    (channel) => channel.id === controller.activeChannelID,
  );
  const repository = controller.repositories.find(
    (item) => item.id === controller.selectedRepositoryID,
  );
  const parent = controller.channels.find((channel) => channel.id === parentID);
  return {
    activeChannel,
    workspaceID: controller.activeWorkspace?.id,
    workspaceName: controller.activeWorkspace?.name,
    channelName: activeChannel?.name,
    parentName: parent?.name,
    agentID: channelAgentID(activeChannel),
    publishAvailable:
      repository?.provider === 'github' && controller.activeRun?.status === 'succeeded',
  };
}

function selectedAgentChatSurface(
  controller: Controller,
  model: ReturnType<typeof pageModel>,
  chat: AgentChatSelection | undefined,
  setChat: (chat: AgentChatSelection | undefined) => void,
  onChange: () => void,
) {
  if (!chat || !model.workspaceID || !controller.threadID) return undefined;
  return (
    <AgentTaskSurface
      key={chat.id}
      api={controller.api}
      workspaceID={model.workspaceID}
      threadID={controller.threadID}
      agentID={chat.agentID}
      resources={controller.repositoryOptions}
      initialResourceID={chat.resourceID}
      taskID={chat.id}
      registered={chat.registered}
      taskRevision={controller.taskRevision}
      onTaskChange={onChange}
      onChatChange={onChange}
      onClose={() => setChat(undefined)}
    />
  );
}

function newAgentChat(
  controller: Controller,
  model: ReturnType<typeof pageModel>,
): AgentChatSelection | undefined {
  const resourceID = controller.selectedRepositoryID ?? controller.repositoryOptions[0]?.id;
  if (!model.agentID || !resourceID) return undefined;
  return {
    id: `local_session_${globalThis.crypto?.randomUUID?.() ?? Date.now().toString(36)}`,
    title: 'New agent chat',
    agentID: model.agentID,
    resourceID,
    registered: false,
  };
}
