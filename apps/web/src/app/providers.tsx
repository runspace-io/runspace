'use client';

import type { ReactNode } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { SessionProvider } from 'next-auth/react';
import { queryClient } from '@/lib/query-client';

export function Providers({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <SessionProvider>
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </SessionProvider>
  );
}
