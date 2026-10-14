import Profile from '@/views/profile'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/profil/')({
  component: Profile,
})


