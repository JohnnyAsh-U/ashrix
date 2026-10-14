import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth.store'
import { Outlet } from '@tanstack/react-router'

const requireGuest = () => {
  const { isAuthenticated } = useAuthStore.getState();
  if (isAuthenticated) throw redirect({ to: '/dashboard' });
};

export const Route = createFileRoute('/_unauthenticated')({
  beforeLoad: requireGuest,
  component: () => <Outlet />,
})  
