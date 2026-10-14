import type { User } from '@/services/auth.service';
import { create } from 'zustand';
import { persist, devtools } from 'zustand/middleware';
// import type { User, AuthTokens } from '../types/auth.types';

interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  isHydrated: boolean;

  // Actions
  setAuth: (user: User, token: string) => void;
  setUser: (user: User) => void;
  clearAuth: () => void;
  setHydrated: () => void;
  updateToken: (token: string) => void;
}

export const useAuthStore = create<AuthState>()(
  devtools(
    persist(
      (set) => ({
        user: null,
        token: null,
        isAuthenticated: false,
        isHydrated: false,

        setAuth: (user, token) =>
          set({ user, token, isAuthenticated: true }, false, 'setAuth'),

        setUser: (user) =>
          set({ user }, false, 'setUser'),

        clearAuth: () =>
          set({ user: null, token: null, isAuthenticated: false }, false, 'clearAuth'),

        setHydrated: () =>
          set({ isHydrated: true }, false, 'setHydrated'),

        updateToken: (token) =>
          set({ token }, false, 'updateTokens'),
      }),
      {
        name: 'ashrix-auth',
        partialize: (state) => ({
          user: state.user,
          token: state.token,
          isAuthenticated: state.isAuthenticated,
        }),
        onRehydrateStorage: () => (state) => {
          state?.setHydrated();
        },
      }
    ),
    { name: 'AshrixAuth' }
  )
);

// Selectors
export const selectUser = (s: AuthState) => s.user;
export const selectIsAuthenticated = (s: AuthState) => s.isAuthenticated;
export const selectTokens = (s: AuthState) => s.token;
export const selectIsHydrated = (s: AuthState) => s.isHydrated;