import { Link, Outlet } from '@tanstack/react-router';
import { MapIcon, MessageCircle } from 'lucide-react';

const navItems = [
  { to: '/map', label: 'Карта', icon: MapIcon },
  { to: '/', label: 'Чат', icon: MessageCircle },
] as const;

export function AppShell() {
  return (
    <div className="flex min-h-screen bg-paper">
      <aside className="flex w-16 flex-col items-center gap-3 border-r border-line py-4">
        <div className="flex h-10 w-10 items-center justify-center bg-ink text-sm font-extrabold text-brand">
          AI
        </div>

        <nav className="mt-3 flex flex-col gap-3">
          {navItems.map(({ to, label, icon: Icon }) => (
            <Link
              key={to}
              to={to}
              title={label}
              aria-label={label}
              activeOptions={{ exact: true }}
              className="flex h-10 w-10 items-center justify-center"
              activeProps={{ className: 'bg-brand text-ink' }}
              inactiveProps={{ className: 'text-muted hover:bg-mist' }}
            >
              <Icon size={22} aria-hidden="true" />
            </Link>
          ))}
        </nav>
      </aside>

      <main className="min-w-0 flex-1 px-7 py-7">
        <Outlet />
      </main>
    </div>
  );
}
