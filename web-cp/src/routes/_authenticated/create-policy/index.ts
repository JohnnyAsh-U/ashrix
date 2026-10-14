import CreatePolicy from '@/views/policy-create'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/create-policy/')({
  component: CreatePolicy,
})
