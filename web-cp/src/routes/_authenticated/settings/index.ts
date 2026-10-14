// import Admins from '@/views/admins'
// import Migrations from '@/views/migrations'
// import Storage from '@/views/storage'
import AdminUsersSettings from '@/views/settings'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/settings/')({
  component: AdminUsersSettings,
})
