import AccessLog from '@/views/logs'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/access-logs/')({
  component: AccessLog,
})
