import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth.store'
import { AuthenticatedLayout } from '@/components/layout/auth.layout'

const requireAuth = () => {
  const { isAuthenticated, isHydrated } = useAuthStore.getState();
  if (!isHydrated) return; // wait for hydration
  if (!isAuthenticated) throw redirect({ to: '/login', search: { redirect: location.pathname } });
};


export const Route = createFileRoute('/_authenticated')({
  beforeLoad: requireAuth,
  component: AuthenticatedLayout,
})
