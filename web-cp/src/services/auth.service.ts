// src/services/auth.service.ts
import api from '@/config/api'
import { ChangePasswordUrl, Disable2FAUrl, Enable2FAUrl, FirstLoginTOTPSetupUrl, FirstLoginTOTPVerifyUrl, LoginUrl, LogoutUrl, MeUrl, Verify2FALoginUrl, Setup2FAUrl } from '@/utils/constant'

export interface User {
  id: string
  username: string
  email: string
  org_id: string
  org_name: string
  is_2fa_enabled: boolean
  phone: string
  role: string
  // add your user fields
}

export interface LoginResponse {
  expires_at: string
  otp_token: string
  totp_setup_required: boolean
}

export interface Setup2FAResponse {
  secret: string
  qr_code_url: string
}

export interface Enable2FAResponse {
  message: string
}

export interface ChangePasswordRequest {
  old_password: string
  new_password: string
}

export interface UpdateProfileRequest {
  email?: string
  phone?: string
  username?: string
}

export const authService = {
  login: async (email: string, password: string): Promise<{success: boolean; data: LoginResponse}> => {
    const { data } = await api.post(LoginUrl, {email, password})
    return data
  },

  verifyTwoFactor: async (tempToken: string, otp_code: string) => {
    const { data } = await api.post(Verify2FALoginUrl, {
      otp_token: tempToken,
      otp_code,
    })
    return data
  },

  setupFirstLoginTOTP: async (otpToken: string): Promise<Setup2FAResponse> => {
    const { data } = await api.post(FirstLoginTOTPSetupUrl, { otp_token: otpToken })
    return data.data
  },

  verifyFirstLoginTOTP: async (otpToken: string, otp_code: string) => {
    const { data } = await api.post(FirstLoginTOTPVerifyUrl, {
      otp_token: otpToken,
      otp_code,
    })
    return data
  },

  setupTwoFactor: async (): Promise<Setup2FAResponse> => {
    const { data } = await api.post(Setup2FAUrl)
    return data
  },

  enableTwoFactor: async (code: string): Promise<Enable2FAResponse> => {
    const { data } = await api.post(Enable2FAUrl, { code })
    return data
  },

  disableTwoFactor: async (code: string): Promise<Enable2FAResponse> => {
    const { data } = await api.post(Disable2FAUrl, { code })
    return data
  },

  changePassword: async (payload: ChangePasswordRequest): Promise<{ message: string }> => {
    const { data } = await api.post(ChangePasswordUrl, payload)
    return data
  },

  updateProfile: async (payload: UpdateProfileRequest): Promise<User> => {
    const { data } = await api.put(MeUrl, payload)
    return data
  },

  logout: async () => {
    await api.get(LogoutUrl)
  },

  me: async (): Promise<{success: boolean; data: User}> => {
    const { data } = await api.get(MeUrl)
    return data
  },
}