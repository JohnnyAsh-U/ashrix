import { SignIn } from '@/views/auth/sign-in'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_unauthenticated/login')({
  component: SignIn,
})