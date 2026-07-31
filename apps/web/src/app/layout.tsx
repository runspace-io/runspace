import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import '@chatscope/chat-ui-kit-styles/dist/default/styles.min.css';
import './globals.css';
import { Providers } from './providers';

export const metadata: Metadata = {
  title: 'Runspace | Agent Engineering Workspace',
  description: 'A focused engineering workspace for human and AI collaboration.',
};

export default function RootLayout({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="en">
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
