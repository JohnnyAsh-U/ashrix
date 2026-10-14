import { useEffect, useRef, useState } from 'react'
import { QRCodeSVG } from 'qrcode.react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useNavigate } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { useCompleteFirstLoginTOTPMutation, useFirstLoginTOTPSetupMutation } from '@/hooks/auth.hooks'

const formSchema = z.object({
  code: z.string().length(6, 'Enter the 6-digit code from your authenticator app'),
})

export function EnrollTOTP({ otpToken }: { otpToken: string }) {
  const navigate = useNavigate()
  const setupMutation = useFirstLoginTOTPSetupMutation()
  const verifyMutation = useCompleteFirstLoginTOTPMutation()
  const [isLoading, setIsLoading] = useState(true)
  const [result, setResult] = useState({qr_code_url: '', secret: ''})
  const started = useRef(false)
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: { code: '' },
  })

  useEffect(() => {
    if (started.current) return

    (async()=>{
    started.current = true;
      console.log("direct call start")
      try{
        const res = await setupMutation.mutateAsync(otpToken)
        setResult(res)
        setIsLoading(false)
      }catch(err){
        setIsLoading(false)
      }
    })()
    
  }, [otpToken])

  async function onSubmit(values: z.infer<typeof formSchema>) {
    try {
      await verifyMutation.mutateAsync({ otpToken, code: values.code })
      toast.success('Authenticator set up')
      navigate({ to: '/', replace: true })
    } catch (error: any) {
      toast.error(error?.response?.data?.error?.details || 'Could not verify the authenticator code')
      form.reset()
    }
  }


  return (
    <div className='grid gap-5'>
      <p className='text-sm text-muted-foreground'>Scan this QR code with your authenticator app, then enter the code it provides.</p>
      {isLoading && <Loader2 className='mx-auto animate-spin' />}
      {setupMutation.isError && (
        <div className='grid gap-3'>
          <p className='text-sm text-destructive'>Could not start authenticator setup. Your login challenge may have expired.</p>
          <Button variant='outline' onClick={() => setupMutation.mutate(otpToken)}>Try again</Button>
        </div>
      )}
      {!isLoading && result.qr_code_url && (
        <>
          <div className='mx-auto rounded-md border bg-white p-3'>
            <QRCodeSVG value={result.qr_code_url} size={192} />
          </div>
          <p className='break-all text-center font-mono text-xs text-muted-foreground'>{result.secret}</p>
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className='grid gap-4'>
              <FormField
                control={form.control}
                name='code'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Authenticator code</FormLabel>
                    <FormControl><Input inputMode='numeric' autoComplete='one-time-code' maxLength={6} {...field} /></FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <Button disabled={verifyMutation.isPending}>
                {verifyMutation.isPending && <Loader2 className='animate-spin' />}
                Verify and continue
              </Button>
            </form>
          </Form>
        </>
      )}
    </div>
  )
}