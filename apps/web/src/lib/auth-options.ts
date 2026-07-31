import type { NextAuthOptions } from 'next-auth';
import CredentialsProvider from 'next-auth/providers/credentials';
import GitHubProvider from 'next-auth/providers/github';

const localProvider = CredentialsProvider({
  id: 'local',
  name: 'Local development',
  credentials: {
    username: { label: 'Username', type: 'text' },
    password: { label: 'Password', type: 'password' },
  },
  authorize: async (credentials) =>
    authorizeLocalUser(credentials?.username, credentials?.password),
});

const localAuthEnabled =
  process.env.NODE_ENV === 'development' || process.env.LOCAL_AUTH_ENABLED === 'true';

export function authorizeLocalUser(
  username: string | undefined,
  password: string | undefined,
  configuration = process.env.LOCAL_AUTH_USERS ?? 'admin:admin,alice:alice',
) {
  const id = username?.trim();
  if (!id || !password) return null;
  const matched = configuration.split(',').some((entry) => {
    const separator = entry.indexOf(':');
    return (
      separator > 0 &&
      entry.slice(0, separator).trim() === id &&
      entry.slice(separator + 1) === password
    );
  });
  if (!matched) return null;
  return {
    id,
    name: id.charAt(0).toUpperCase() + id.slice(1),
    email: `${id}@runspace.local`,
  };
}

export const authOptions: NextAuthOptions = {
  providers: [
    ...(localAuthEnabled ? [localProvider] : []),
    ...(process.env.GITHUB_ID && process.env.GITHUB_SECRET
      ? [
          GitHubProvider({
            clientId: process.env.GITHUB_ID,
            clientSecret: process.env.GITHUB_SECRET,
          }),
        ]
      : []),
  ],
  session: { strategy: 'jwt' },
  secret: process.env.NEXTAUTH_SECRET ?? 'runspace-development-secret-change-me',
  pages: { signIn: '/signin' },
  callbacks: {
    session({ session, token }) {
      if (session.user && token.sub) session.user.id = token.sub;
      return session;
    },
  },
};
