'use client';

import { signOut, useSession } from 'next-auth/react';
import { WorkspacePageShell } from '@/components/workspace-page-shell';
import { useWorkspaceController } from '@/components/use-workspace-controller';

export default function HomePage() {
  const { data: session } = useSession();
  const userID = sessionUserID(session?.user?.id);
  const controller = useWorkspaceController(userID);
  return (
    <WorkspacePageShell
      controller={controller}
      onSignOut={() => void signOut({ callbackUrl: '/signin' })}
    />
  );
}

function sessionUserID(id: string | undefined): string {
  if (id) return id;
  return process.env.NODE_ENV === 'development' ? 'admin' : '';
}
