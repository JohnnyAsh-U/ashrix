// import { useNavigate } from '@tanstack/react-router'
import { useLoginMutation } from '@/hooks/auth.hooks'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import {
    Form,
    FormControl,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { PasswordInput } from '@/components/password-input'
import { Link, Loader2, LogIn } from 'lucide-react'
import { Button } from '@/components/ui/button'


type Props = {
    setPage: React.Dispatch<React.SetStateAction<'login' | '2fa' | 'totp-enroll'>>
    setToken: React.Dispatch<React.SetStateAction<string>>
}

const formSchema = z.object({
    email: z
        .string()
        .min(1, 'Please enter your email'),
    password: z
        .string()
        .min(1, 'Please enter your password')
        .min(4, 'Password must be at least 4 characters long'),
})

export default function LoginForm({ setPage, setToken }: Props) {
    // const navigate = useNavigate()
    const loginMutation = useLoginMutation()

    const form = useForm<z.infer<typeof formSchema>>({
        resolver: zodResolver(formSchema),
        defaultValues: {
            email: '',
            password: '',
        },
    })

    async function onSubmit(data: z.infer<typeof formSchema>) {
        try {
            const result = await loginMutation.mutateAsync({
                email: data.email,
                password: data.password,
            })
            setToken(result.data.otp_token)
            setPage(result.data.totp_setup_required ? 'totp-enroll' : '2fa')
        } catch (error: any) {
            const errorMessage = error?.response?.data?.error.details || 'Login failed'
            console.error('Login error:', error)
            toast.error(errorMessage)
        }
    }

    return (
        <Form {...form}>
            <form
                onSubmit={form.handleSubmit(onSubmit)}
                className={'grid gap-3'}

            >
                <FormField
                    control={form.control}
                    name='email'
                    render={({ field }) => (
                        <FormItem>
                            <FormLabel>Email</FormLabel>
                            <FormControl>
                                <Input placeholder='your email' {...field} />
                            </FormControl>
                            <FormMessage />
                        </FormItem>
                    )}
                />
                <FormField
                    control={form.control}
                    name='password'
                    render={({ field }) => (
                        <FormItem className='relative'>
                            <FormLabel>Password</FormLabel>
                            <FormControl>
                                <PasswordInput placeholder='********' {...field} />
                            </FormControl>
                            <FormMessage />
                            <Link
                                to='/forgot-password'
                                className='absolute inset-e-0 -top-0.5 text-sm font-medium text-muted-foreground hover:opacity-75'
                            >
                                Forgot password?
                            </Link>
                        </FormItem>
                    )}
                />
                <Button className='mt-2' disabled={loginMutation.isPending}>
                    {loginMutation.isPending ? (
                        <Loader2 className='animate-spin' />
                    ) : (
                        <LogIn />
                    )}
                    Sign in
                </Button>
            </form>
        </Form>
    )
}