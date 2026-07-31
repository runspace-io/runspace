'use client';

import { useEffect, useState, type FormEvent } from 'react';
import { getProviders, signIn } from 'next-auth/react';
import Image from 'next/image';

export default function SignInPage() {
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('admin');
  const [githubAvailable, setGithubAvailable] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string>();
  useEffect(() => {
    void getProviders().then((providers) => setGithubAvailable(Boolean(providers?.github)));
  }, []);
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitting(true);
    setError(undefined);
    const result = await signIn('local', {
      username,
      password,
      redirect: false,
      callbackUrl: '/',
    });
    setSubmitting(false);
    if (!result?.ok) {
      setError('Invalid username or password.');
      return;
    }
    window.location.assign(result.url ?? '/');
  };
  return (
    <main className="auth-page">
      <div className="auth-card">
        <Image src="/brand/runspace-icon.svg" width={34} height={34} alt="" />
        <p className="eyebrow">RUNSPACE / AUTHENTICATION</p>
        <h1>Enter your workspace</h1>
        <p className="auth-copy">Sign in to build and collaborate in your local Runspace.</p>
        <form className="auth-form" onSubmit={submit}>
          <label>
            <span>Username</span>
            <input
              autoComplete="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
            />
          </label>
          <label>
            <span>Password</span>
            <input
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </label>
          <button className="dialog-primary" type="submit" disabled={submitting}>
            {submitting ? 'Signing in…' : 'Sign in'}
          </button>
        </form>
        {githubAvailable && (
          <button
            className="quiet-button local-auth-button"
            type="button"
            onClick={() => void signIn('github', { callbackUrl: '/' })}
          >
            Continue with GitHub <span>→</span>
          </button>
        )}
        {error && (
          <p className="auth-error" role="alert">
            {error}
          </p>
        )}
      </div>
    </main>
  );
}
