import Gateways from '@/views/gateways'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/gateways/')({
  component: Gateways,
})
