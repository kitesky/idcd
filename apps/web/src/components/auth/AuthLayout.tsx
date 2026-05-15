import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui"
import Link from "next/link"

interface AuthLayoutProps {
  children: React.ReactNode
  title: string
  description?: string
  footer?: React.ReactNode
}

export function AuthLayout({ children, title, description, footer }: AuthLayoutProps) {
  return (
    <main className="flex-1 flex items-center justify-center p-4">
      <div className="w-full max-w-md space-y-4">
        <div className="text-center mb-6">
          <Link href="/" className="inline-block">
            <h1 className="text-3xl font-bold text-primary">idcd</h1>
          </Link>
        </div>
        <Card>
          <CardHeader className="space-y-1 pb-4 pt-6 px-6">
            <CardTitle className="text-2xl">{title}</CardTitle>
            {description && <CardDescription className="mt-1">{description}</CardDescription>}
          </CardHeader>
          <CardContent className="px-6 pb-6">
            {children}
          </CardContent>
        </Card>
        {footer && (
          <div className="text-center text-sm text-muted-foreground">
            {footer}
          </div>
        )}
      </div>
    </main>
  )
}
