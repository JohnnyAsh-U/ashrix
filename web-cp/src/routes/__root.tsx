import * as React from 'react'
import type { QueryClient } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { Outlet, createRootRouteWithContext } from '@tanstack/react-router'
import { NavigationProgress } from '@/components/navigation-progress'
import { Toaster } from '@/components/ui/sonner'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'
import { NotFoundError } from '@/views/errors/not-found-error'
import { GeneralError } from '@/views/errors/general-error'


export interface MyRouterContext {
  queryClient: QueryClient
}

export const Route = createRootRouteWithContext<MyRouterContext>()({
  component: RootComponent,
  notFoundComponent: NotFoundError,
  errorComponent: GeneralError
})

function RootComponent() {
  return (
    <React.Fragment>
      <NavigationProgress />
        <Outlet />
      <Toaster duration={3000} />
      <TanStackRouterDevtools position='bottom-right' />
      <ReactQueryDevtools initialIsOpen={false} buttonPosition='bottom-left' />
    </React.Fragment>
  )
}
