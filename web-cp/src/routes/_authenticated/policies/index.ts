import Policies from '@/views/policies'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/policies/')({
  component: Policies,
})
