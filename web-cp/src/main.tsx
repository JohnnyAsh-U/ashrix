import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { routeTree } from './routeTree.gen'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import './styles/index.css'
import { ThemeProvider } from './context/theme-provider'
import { QueryClientProvider, QueryClient } from '@tanstack/react-query'
import { AuthProvider } from './context/auth-provider'


const queryClient = new QueryClient()


const router = createRouter({
  routeTree,
  context: { queryClient },
  defaultPreload: 'intent',
  defaultPreloadStaleTime: 0
})


declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

const rootElement = document.getElementById('root')!

if (!rootElement.innerHTML) {
  createRoot(rootElement).render(
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <ThemeProvider>
            <RouterProvider router={router} context={{ queryClient }} />
          </ThemeProvider>
        </AuthProvider>
      </QueryClientProvider>
    </StrictMode>,
  )
}


