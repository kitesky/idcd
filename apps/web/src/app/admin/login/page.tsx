import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { loginAction } from "./actions"

interface PageProps {
  searchParams: Promise<{ reason?: string; next?: string }>
}

export default async function AdminLoginPage({ searchParams }: PageProps) {
  const sp = await searchParams
  const reason = sp.reason
  const next = sp.next ?? "/admin"

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-6">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>idcd Admin</CardTitle>
        </CardHeader>
        <CardContent>
          {reason === "not_configured" && (
            <Alert variant="destructive" className="mb-4">
              <AlertDescription>
                ADMIN_PORTAL_TOKEN not set on the server. Configure the env var and redeploy.
              </AlertDescription>
            </Alert>
          )}
          {reason === "invalid_token" && (
            <Alert variant="destructive" className="mb-4">
              <AlertDescription>Invalid token.</AlertDescription>
            </Alert>
          )}
          {reason === "missing_token" && (
            <Alert variant="destructive" className="mb-4">
              <AlertDescription>Token required.</AlertDescription>
            </Alert>
          )}
          <form action={loginAction} className="flex flex-col gap-4">
            <input type="hidden" name="next" value={next} />
            <div className="flex flex-col gap-2">
              <Label htmlFor="token">Portal token</Label>
              <Input id="token" name="token" type="password" autoComplete="off" required />
            </div>
            <Button type="submit">Sign in</Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
