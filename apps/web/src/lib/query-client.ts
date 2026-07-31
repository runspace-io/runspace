import { QueryClient } from '@tanstack/react-query';
import { retryDelay } from './retry-policy';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      networkMode: 'online',
      retry: 10,
      retryDelay,
      refetchOnReconnect: true,
      refetchOnWindowFocus: true,
      staleTime: 5_000,
    },
    mutations: {
      retry: 5,
      retryDelay,
      networkMode: 'online',
    },
  },
});
