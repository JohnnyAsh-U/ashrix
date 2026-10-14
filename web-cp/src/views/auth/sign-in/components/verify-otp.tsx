import React from 'react'

import { useEffect, useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useNavigate } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormMessage,
} from '@/components/ui/form'
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSlot,
} from '@/components/ui/input-otp'
import { useVerify2FAMutation } from '@/hooks/auth.hooks'

const formSchema = z.object({
  code: z
    .string()
    .min(6, 'Veuillez entrer le code à 6 chiffres')
    .max(8, 'Le code doit contenir au maximum 8 caractères'),
})

interface Verify2FAFormProps extends React.HTMLAttributes<HTMLFormElement> {
  tempToken: string
  redirectTo?: string
}

export function Verify2FAForm({
  tempToken,
  redirectTo,
  ...props
}: Verify2FAFormProps) {
  const navigate = useNavigate()
  const verify2FAMutation = useVerify2FAMutation()
  const [timer, setTimer] = useState(300) // 5 minutes

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      code: '',
    },
  })

  useEffect(() => {
    const interval = setInterval(() => {
      setTimer((prev) => {
        if (prev <= 1) {
          clearInterval(interval)
          toast.error('Session expirée, veuillez vous reconnecter')
          navigate({ to: '/login', replace: true })
          return 0
        }
        return prev - 1
      })
    }, 2000)

    return () => clearInterval(interval)
  }, [])

  async function onSubmit(data: z.infer<typeof formSchema>) {
    try {
      await verify2FAMutation.mutateAsync({
        tempToken,
        code: data.code,
      })

      toast.success('Authentification à deux facteurs vérifiée')
      navigate({
        to: '/',
        replace: true,
      })
    } catch (error: any) {
      const errorMessage = error?.response?.data?.details || 'Échec de la vérification 2FA'
      toast.error(errorMessage)
      form.reset()
    }
  }

  const minutes = Math.floor(timer / 60)
  const seconds = timer % 60

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-4')}
        {...props}
      >
        <FormField
          control={form.control}
          name='code'
          render={({ field }) => (
            <FormItem>
              <FormControl className='w-full'>
                <div className='flex justify-center'>
                  <InputOTP maxLength={6} {...field}>
                    <InputOTPGroup>
                      <InputOTPSlot index={0} />
                      <InputOTPSlot index={1} />
                      <InputOTPSlot index={2} />
                      <InputOTPSlot index={3} />
                      <InputOTPSlot index={4} />
                      <InputOTPSlot index={5} />
                    </InputOTPGroup>
                  </InputOTP>
                </div>
              </FormControl>
              <FormMessage />
              <p className='text-xs text-muted-foreground mt-2'>
                Entrez le code à 6 chiffres de votre application d'authentification
              </p>
            </FormItem>
          )}
        />

        <div className='text-center text-sm text-muted-foreground'>
          Session expire dans {minutes}:{seconds.toString().padStart(2, '0')}
        </div>

        <Button disabled={verify2FAMutation.isPending}>
          {verify2FAMutation.isPending && (
            <Loader2 className='animate-spin' />
          )}
          Verifier
        </Button>
      </form>
    </Form>
  )
}
