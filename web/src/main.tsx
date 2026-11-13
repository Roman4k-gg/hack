import React from 'react';
import ReactDOM from 'react-dom/client';
import {
  MutationCache,
  QueryCache,
  QueryClient,
  QueryClientProvider,
} from '@tanstack/react-query';
import { RouterProvider, createRouter } from '@tanstack/react-router';
import { useAuthStore } from '@/features/auth/authStore';
import { ApiError } from '@/lib/api';
import { routeTree } from './routeTree.gen';
import './index.css';

const router = createRouter({ routeTree });

function handleUnauthorized(error: unknown) {
  const { token, logout } = useAuthStore.getState();

  if (error instanceof ApiError && error.status === 401 && token) {
    logout();
    queryClient.clear();
    void router.navigate({ to: '/login' });
  }
}

const queryClient = new QueryClient({
  queryCache: new QueryCache({ onError: handleUnauthorized }),
  mutationCache: new MutationCache({ onError: handleUnauthorized }),
  defaultOptions: {
    queries: {
      retry: (failureCount, error) =>
        !(error instanceof ApiError && error.status < 500) && failureCount < 2,
    },
  },
});

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </React.StrictMode>,
);