'use client';

import { Panel, PanelGroup, PanelResizeHandle } from 'react-resizable-panels';
import type { ReactNode } from 'react';

export function ResizableWorkspacePanels({
  navigation,
  main,
}: {
  navigation: ReactNode;
  main: ReactNode;
}) {
  return (
    <PanelGroup
      direction="horizontal"
      autoSaveId="runspace-workspace-panels"
      className="workspace-grid"
    >
      <Panel
        id="navigation"
        order={1}
        defaultSize={18}
        minSize={14}
        maxSize={28}
        className="workspace-panel navigation-panel"
      >
        {navigation}
      </Panel>
      <PanelResizeHandle className="panel-resize-handle" aria-label="Resize navigation panel" />
      <Panel
        id="main"
        order={2}
        defaultSize={82}
        minSize={40}
        className="workspace-panel main-panel"
      >
        {main}
      </Panel>
    </PanelGroup>
  );
}
