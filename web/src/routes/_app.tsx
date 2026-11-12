import { createFileRoute, redirect } from '@tanstack/react-router';
import { AppShell } from '@/app/AppShell';
import { useAuthStore } from '@/features/auth/authStore';

export const Route = createFileRoute('/_app')({
  beforeLoad: () => {
    if (!useAuthStore.getState().token) {
      throw redirect({ to: '/login' });
    }
  },
  component: AppShell,
});
