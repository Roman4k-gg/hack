import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { LoginResponse, User } from './types';

type AuthState = {
  token: string | null;
  user: User | null;
  setSession: (session: LoginResponse) => void;
  logout: () => void;
};

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      user: null,
      setSession: (session) =>
        set({ token: session.token, user: session.user }),
      logout: () => set({ token: null, user: null }),
    }),
    { name: 'ai-buddy-auth' },
  ),
);
