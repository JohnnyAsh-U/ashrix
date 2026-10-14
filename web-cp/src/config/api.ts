// src/config/api.ts
import axios from 'axios'
import { jwtDecode, type JwtPayload } from 'jwt-decode'
import { useAuthStore } from '@/stores/auth.store'
import { RefreshTokenUrl } from '@/utils/constant'

const refreshClient = axios.create({
  baseURL: import.meta.env.VITE_API_URL,
  withCredentials: true,
})

type RefreshResponse = {
  data: {
    access_token: string
  }
}

let refreshPromise: Promise<string> | null = null

const decodeTokenExpiry = (token: string): number | null => {
  try {
    return jwtDecode<JwtPayload>(token).exp ?? null
  } catch {
    return null
  }
}

const tokenNeedsRefresh = (token: string) => {
  const expiry = decodeTokenExpiry(token)
  return expiry !== null && expiry * 1000 - Date.now() <= 30_000
}

const refreshAccessToken = async (): Promise<string> => {
  if (!refreshPromise) {
    refreshPromise = refreshClient
      .post<RefreshResponse>(RefreshTokenUrl)
      .then(({ data }) => {
        const token = data.data.access_token
        useAuthStore.getState().updateToken(token)
        return token
      })
      .finally(() => {
        refreshPromise = null
      })
  }

  return refreshPromise
}

const clearAuthAndRedirect = () => {
  useAuthStore.getState().clearAuth()
  if (window.location.pathname !== '/login') {
    window.location.replace('/login')
  }
}

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL,
  withCredentials: true,
})

api.interceptors.request.use(async (config) => {
  let token = useAuthStore.getState().token
  if (token && tokenNeedsRefresh(token)) {
    try {
      token = await refreshAccessToken()
    } catch {
      clearAuthAndRedirect()
      return Promise.reject(new axios.CanceledError('Session expirée'))
    }
  }

  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

api.interceptors.response.use(
  (res) => res,
  (error) => {
    if (error.response?.status === 401 && error.config?.url !== RefreshTokenUrl) {
      clearAuthAndRedirect()
    }
    return Promise.reject(error)
  }
)

export default api