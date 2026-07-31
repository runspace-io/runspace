'use client';

import { Panel, PanelGroup, PanelResizeHandle } from 'react-resizable-panels';
import type { ReactNode } from 'react';

export function ResizableWorkspacePanels({
  navigation,
  main,
  details,
}: {
  navigation: ReactNode;
  main: ReactNode;
  details: ReactNode;
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
        defaultSize={64}
        minSize={40}
        className="workspace-panel main-panel"
      >
        {main}
      </Panel>
      {details ? (
        <>
          <PanelResizeHandle className="panel-resize-handle" aria-label="Resize details panel" />
          <Panel
            id="details"
            order={3}
            defaultSize={18}
            minSize={16}
            maxSize={28}
            className="workspace-panel details-panel"
          >
            {details}
          </Panel>
        </>
      ) : null}
    </PanelGroup>
  );
}
