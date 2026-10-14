// src/hooks/auth.hooks.ts
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { authService } from '@/services/auth.service'
import { useAuthStore } from '@/stores/auth.store'
import { useNavigate } from '@tanstack/react-router'

// ---- Bootstrap: who am I? ----
export const useMeQuery = () => {
  const { setUser, token } = useAuthStore()

  return useQuery({
    queryKey: ['auth', 'me'],
    queryFn: async () => {
      const data = await authService.me()
      setUser(data.data)
      return data.data
    },
    enabled: !!token,             // only runs if token exists
    retry: false,
    staleTime: 5 * 60 * 1000,    // 5 min
    throwOnError: false,
  })
}

// ---- Login ----
export const useLoginMutation = () => {
  // const { setUser, setAuth } = useAuthStore()
  // const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ email, password }: { email: string; password: string }) =>
      authService.login(email, password),

    // onSuccess: (data) => {
      // if (data.requires_2fa) return // caller handles 2FA redirect

      // setAuth(data.user!, data.access_token!)
      // setUser(data.user!)
      // queryClient.invalidateQueries({ queryKey: ['auth'] })
    // },
  })
}

// ---- 2FA Verify ----
export const useVerify2FAMutation = () => {
  const { updateToken, setUser, setAuth } = useAuthStore()
  const queryClient = useQueryClient()


  return useMutation({
    mutationFn: ({ tempToken, code }: { tempToken: string; code: string }) =>
      authService.verifyTwoFactor(tempToken, code),

    onSuccess: (data) => {
      setAuth(data.data.admin!, data.data.tokens.access_token!)
      updateToken(data.data.tokens.access_token)
      setUser(data.data.admin)
      queryClient.invalidateQueries({ queryKey: ['auth'] })
    },
  })
}

export const useFirstLoginTOTPSetupMutation = () => {
  return useMutation({
    mutationFn: (otpToken: string) => authService.setupFirstLoginTOTP(otpToken),
  })
}

export const useCompleteFirstLoginTOTPMutation = () => {
  const { updateToken, setUser, setAuth } = useAuthStore()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ otpToken, code }: { otpToken: string; code: string }) =>
      authService.verifyFirstLoginTOTP(otpToken, code),
    onSuccess: (response) => {
      const data = response.data
      setAuth(data.admin, data.tokens.access_token)
      updateToken(data.tokens.access_token)
      setUser(data.admin)
      queryClient.invalidateQueries({ queryKey: ['auth'] })
    },
  })
}

// ---- 2FA Setup ----
export const useSetup2FAMutation = () => {
  return useMutation({
    mutationFn: () => authService.setupTwoFactor(),
  })
}

// ---- 2FA Enable ----
export const useEnable2FAMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (code: string) => authService.enableTwoFactor(code),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['auth', 'me'] })
    },
  })
}

// ---- 2FA Disable ----
export const useDisable2FAMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (code: string) => authService.disableTwoFactor(code),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['auth', 'me'] })
    },
  })
}

// ---- Change Password ----
export const useChangePasswordMutation = () => {
  return useMutation({
    mutationFn: (payload: { old_password: string; new_password: string }) =>
      authService.changePassword(payload),
  })
}

// ---- Update Profile ----
export const useUpdateProfileMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (payload: { email?: string; phone?: string; username?: string }) =>
      authService.updateProfile(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['auth', 'me'] })
    },
  })
}

// ---- Logout ----
export const useLogoutMutation = () => {
  const { clearAuth } = useAuthStore()
  const queryClient = useQueryClient()
    const navigate = useNavigate()
  

  return useMutation({
    mutationFn: authService.logout,
    onSettled: () => {
      clearAuth()                              // clear zustand + localStorage
      queryClient.clear()                   // wipe all cached queries
      navigate({
        to: '/login',
        replace: true,
      })
    },
  })
}