import { Link, Outlet, useNavigate } from '@tanstack/react-router';
import { LogOut, MapIcon, MessageCircle } from 'lucide-react';
import { useAuthStore } from '@/features/auth/authStore';

const navItems = [
  { to: '/map', label: 'Карта', icon: MapIcon, color: 'bg-brand' },
  { to: '/', label: 'Чат', icon: MessageCircle, color: 'bg-azure' },
] as const;

export function AppShell() {
  const navigate = useNavigate();
  const user = useAuthStore((state) => state.user);
  const logout = useAuthStore((state) => state.logout);

  function onLogout() {
    logout();
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
        <Outlet />
      </main>
    </div>
  );
}
