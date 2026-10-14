import UpdatePolicy from '@/views/policy-update'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/update-policy/$id')({
  component: UpdatePolicy,
})
