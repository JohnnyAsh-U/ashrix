import { useMeQuery } from "@/hooks/auth.hooks"
import { Spinner } from "@/components/ui/spinner"

export const AuthProvider = ({
  children,
}: {
  children: React.ReactNode
}) => {
  const { isLoading } = useMeQuery()

  if (isLoading) {
    return (
      <div className="flex h-screen w-screen items-center justify-center">
        <Spinner className="size-8" />
      </div>
    )
  }

  return <>{children}</>
}