// import Admins from '@/views/admins'
import Applications from '@/views/applications'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/applications/')({
  component: Applications,
})
