import Connectors from '@/views/connectors'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/connectors/')({
  component: Connectors,
})
