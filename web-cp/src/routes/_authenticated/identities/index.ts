import IdentitySettings from '@/views/identities'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/identities/')({
  component: IdentitySettings,
})
