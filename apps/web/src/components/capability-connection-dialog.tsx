'use client';

import { Laptop, PlugZap, Server } from 'lucide-react';
import { useState, type ReactNode } from 'react';
import type { WorkspaceApiClient } from '@/lib/api-client';
import { ConnectionDialog, ConnectionMethodPicker } from './connection-dialog';
import { LocalResourceConnectionForm } from './local-resource-connection-form';
import { NativeResourceConnectionForm } from './native-resource-connection-form';

type Placement = 'native' | 'local';

const placements = [
  {
    id: 'native',
    label: 'Runspace plugin',
    description: 'Always available; no computer required',
    icon: <Server size={18} />,
  },
  {
    id: 'local',
    label: 'Local connector',
    description: 'Uses credentials already on this computer',
    icon: <Laptop size={18} />,
  },
] satisfies Array<{
  id: Placement;
  label: string;
  description: string;
  icon: ReactNode;
}>;

export function CapabilityConnectionDialog({
  api,
  userID,
  workspaceID,
  onConnected,
  onClose,
}: {
  api: WorkspaceApiClient;
  userID: string;
  workspaceID: string;
  onConnected: () => void;
  onClose: () => void;
}) {
  const [placement, setPlacement] = useState<Placement>('native');
  return (
    <ConnectionDialog
      eyebrow="Workspace Resource"
      title="Connect a system"
      description="Choose where the integration runs. Native plugins remain available when your computer is offline."
      icon={<PlugZap size={24} />}
      onClose={onClose}
    >
      <ConnectionMethodPicker
        label="Execution placement"
        methods={placements}
        value={placement}
        onChange={setPlacement}
      />
      {placement === 'native' ? (
        <NativeResourceConnectionForm
          api={api}
          workspaceID={workspaceID}
          onConnected={onConnected}
        />
      ) : (
        <LocalResourceConnectionForm
          userID={userID}
          workspaceID={workspaceID}
          onConnected={onConnected}
        />
      )}
    </ConnectionDialog>
  );
}
