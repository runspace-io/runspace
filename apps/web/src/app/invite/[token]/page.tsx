'use client';

import { use, useCallback, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useSession } from 'next-auth/react';
import { WorkspaceApiClient, type ApiInvitationPreview } from '@/lib/api-client';

type State =
  | { status: 'loading' }
  | { status: 'ready'; preview: ApiInvitationPreview }
  | { status: 'joined'; preview: ApiInvitationPreview }
  | { status: 'invalid' };

/**
 * Redeems an invitation link.
 *
 * The middleware already sends signed-out visitors to /signin and back here, so
 * by the time this renders the visitor has an identity. That identity is taken
 * from the session server-side — the token says which workspace to join, never
 * who is joining.
 */
export default function InvitePage({ params }: { params: Promise<{ token: string }> }) {
  const { token } = use(params);
  const router = useRouter();
  const { data: session, status: sessionStatus } = useSession();
  const [state, setState] = useState<State>({ status: 'loading' });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const userID = session?.user?.id;

  useEffect(() => {
    if (sessionStatus === 'loading' || !userID) return;
    let active = true;
    const api = new WorkspaceApiClient({ userID });
    void api
      .previewInvitation(token)
      .then((preview) => active && setState({ status: 'ready', preview }))
      .catch(() => active && setState({ status: 'invalid' }));
    return () => {
      active = false;
    };
  }, [token, userID, sessionStatus]);

  const join = useCallback(async () => {
    if (state.status !== 'ready' || !userID || busy) return;
    setBusy(true);
    setError(undefined);
    try {
      await new WorkspaceApiClient({ userID }).acceptInvitation(token);
      setState({ status: 'joined', preview: state.preview });
      router.push('/');
    } catch {
      // The link is single use, so the usual failure is that it is already spent.
      setError('This invitation is no longer valid. Ask for a fresh link.');
    } finally {
      setBusy(false);
    }
  }, [busy, router, state, token, userID]);

  return (
    <main className="invite-page">
      <section className="invite-card">
        <p className="eyebrow">WORKSPACE INVITATION</p>
        <InviteBody state={state} busy={busy} onJoin={() => void join()} />
        {error && (
          <p className="dialog-error" role="alert">
            {error}
          </p>
        )}
      </section>
    </main>
  );
}

function InviteBody({ state, busy, onJoin }: { state: State; busy: boolean; onJoin: () => void }) {
  if (state.status === 'loading') return <h1>Checking this invitation…</h1>;
  if (state.status === 'invalid') {
    return (
      <>
        <h1>This invitation cannot be used</h1>
        <p>It may have been used already, revoked, or expired.</p>
      </>
    );
  }
  const joined = state.status === 'joined';
  return (
    <>
      <h1>Join {state.preview.workspace_name}</h1>
      <p>
        {state.preview.invited_by} invited you as a <strong>{state.preview.role}</strong>.
      </p>
      <button className="dialog-primary" type="button" disabled={busy || joined} onClick={onJoin}>
        {joined ? 'Joined' : busy ? 'Joining…' : 'Join workspace'}
      </button>
    </>
  );
}
