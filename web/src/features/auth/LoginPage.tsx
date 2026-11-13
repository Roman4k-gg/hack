import { useState, type FormEvent } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { login } from './api';
import { useAuthStore } from './authStore';
import { demoUsers } from './demoUsers';
import { roleLabels } from './types';

export function LoginPage() {
  const navigate = useNavigate();
  const setSession = useAuthStore((state) => state.setSession);
  const [email, setEmail] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function signIn(value: string) {
    setLoading(true);
    setError('');

    try {
      const session = await login(value);
      setSession(session);

      const home = session.user.role === 'intern' ? '/map' : '/dashboard';
      await navigate({ to: home });
    } catch {
      setError('Не удалось войти. Проверьте почту.');
    } finally {
      setLoading(false);
    }
  }

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    void signIn(email);
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-mist p-7">
      <div className="w-full max-w-lg bg-paper p-8 text-center">
        <div className="flex items-center justify-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center bg-ink text-sm font-extrabold text-brand">
            AI
          </div>
          <span className="text-xl font-extrabold tracking-tight">buddy</span>
        </div>

        <h1 className="mt-6 text-2xl font-extrabold tracking-tight">Вход</h1>
        <p className="mt-1 whitespace-nowrap text-sm text-muted">
          Введите рабочую почту, чтобы открыть карту онбординга.
        </p>

        <form
          onSubmit={onSubmit}
          className="mt-5 flex flex-col gap-3 text-left"
        >
          <input
            type="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            placeholder="name@demo.local"
            aria-label="Почта"
            required
            className="border border-line px-3 py-2 text-sm outline-none focus:border-ink"
          />
          <button
            type="submit"
            disabled={loading}
            className="bg-brand px-4 py-2 text-sm font-bold text-ink disabled:opacity-50"
          >
            {loading ? 'Входим…' : 'Войти'}
          </button>
          {error && <p className="text-sm text-alert">{error}</p>}
        </form>

        <div className="mt-6 border-t border-line pt-4">
          <p className="text-xs font-bold text-muted">Демо-аккаунты</p>
          <ul className="mt-2 flex flex-col gap-2 text-left">
            {demoUsers.map((user) => (
              <li key={user.id}>
                <button
                  type="button"
                  disabled={loading}
                  onClick={() => void signIn(user.email)}
                  className="w-full bg-mist px-3 py-2 text-left text-sm hover:bg-line"
                >
                  <span className="font-bold">{user.full_name}</span>
                  <span className="text-muted"> · {roleLabels[user.role]}</span>
                </button>
              </li>
            ))}
          </ul>
        </div>

        <p className="mt-6 text-xs text-muted">
          Кейс Hack&amp;Change 2026 · Avito Start
        </p>
      </div>
    </main>
  );
}
