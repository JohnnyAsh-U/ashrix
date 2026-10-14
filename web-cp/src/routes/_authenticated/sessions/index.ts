// import Admins from '@/views/admins'
// import Migrations from '@/views/migrations'
// import Storage from '@/views/storage'
import Sessions from '@/views/sessions'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/sessions/')({
  component: Sessions,
})
