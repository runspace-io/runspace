import type { useWorkspaceController } from './use-workspace-controller';
import { WorkspaceTopbar } from './workspace-topbar';

type Controller = ReturnType<typeof useWorkspaceController>;

export function WorkspacePageTopbar({
  controller,
  workspaceName,
  onOpenMembers,
  onSignOut,
}: {
  controller: Controller;
  workspaceName: string | undefined;
  onOpenMembers: () => void;
  onSignOut: () => void;
}) {
  return (
    <WorkspaceTopbar
      workspaceName={workspaceName}
      connected={controller.realtimeStatus === 'connected'}
      onOpenNavigation={() => controller.setLeftOpen(true)}
      onOpenWorkspace={() => controller.openDialog('list')}
      onOpenMembers={onOpenMembers}
      onSignOut={onSignOut}
    />
  );
}
