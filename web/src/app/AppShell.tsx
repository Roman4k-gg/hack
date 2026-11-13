import { useQueryClient } from '@tanstack/react-query';
import { Link, Outlet, useNavigate } from '@tanstack/react-router';
import {
  ClipboardList,
  LogOut,
  MapIcon,
  MessageCircle,
  Users,
} from 'lucide-react';
import { useAuthStore } from '@/features/auth/authStore';
import { roleLabels } from '@/features/auth/types';

const TEAM_SCOPE = 'onboarding:team:read';

export function AppShell() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useAuthStore((state) => state.user);
  const logout = useAuthStore((state) => state.logout);

  const scopes = user?.scopes ?? [];
  const canSeeTeam = scopes.includes(TEAM_SCOPE) || scopes.includes('*');

  const navItems = [
    { to: '/map', label: 'Карта', icon: MapIcon, color: 'bg-brand-soft' },
    { to: '/', label: 'Чат', icon: MessageCircle, color: 'bg-azure-soft' },
    {
      to: '/requests',
      label: 'Заявки',
      icon: ClipboardList,
      color: 'bg-alert-soft',
    },
    ...(canSeeTeam
      ? [
          {
            to: '/dashboard',
            label: 'Команда',
            icon: Users,
            color: 'bg-iris-soft',
          },
        ]
      : []),
  ];

  function onLogout() {
    logout();
    queryClient.clear();
    void navigate({ to: '/login' });
  }

  return (
    <div className="flex min-h-screen bg-paper">
      <aside className="flex w-16 flex-col items-center gap-3 border-r border-line py-4">
        <div className="flex h-10 w-10 items-center justify-center bg-ink text-sm font-extrabold text-brand">
          AI
        </div>

        <nav className="mt-3 flex flex-col gap-3">
          {navItems.map(({ to, label, icon: Icon, color }) => (
            <Link
              key={to}
              to={to}
              title={label}
              aria-label={label}
              activeOptions={{ exact: true }}
              className={`flex h-10 w-10 items-center justify-center text-ink ${color}`}
              activeProps={{ className: 'ring-2 ring-ink ring-offset-2' }}
            >
              <Icon size={22} aria-hidden="true" />
            </Link>
          ))}
        </nav>

        <button
          type="button"
          onClick={onLogout}
          title={user ? `Выйти (${user.full_name})` : 'Выйти'}
          aria-label="Выйти"
          className="mt-auto flex h-10 w-10 items-center justify-center bg-mist text-muted hover:bg-line"
        >
          <LogOut size={22} aria-hidden="true" />
        </button>
      </aside>

      <main className="flex min-w-0 flex-1 flex-col px-7 py-7">
        {user && (
          <header className="mb-5 flex items-baseline justify-between gap-4 border-b border-line pb-3">
            <p className="text-sm font-bold">{user.full_name}</p>
            <p className="text-xs text-muted">
              {roleLabels[user.role]}
              {user.team_name ? ` · ${user.team_name}` : ''}
            </p>
          </header>
        )}
        <Outlet />
      </main>
    </div>
  );
}