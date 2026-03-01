import { useState } from "react"
import { Link } from "react-router"
import { AirplaneTakeoff, WarningCircle } from "@phosphor-icons/react"

import { Alert, AlertDescription } from "~/components/ui/alert"
import { Button } from "~/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card"
import { Input } from "~/components/ui/input"
import { Label } from "~/components/ui/label"
import { ApiError } from "~/lib/api"
import { ROUTES } from "~/constants"

import { useAuth } from "~/contexts"

export default function RegisterPage() {
  const { register } = useAuth()
  const [name, setName] = useState("")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")
  const [isSubmitting, setIsSubmitting] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError("")
    setIsSubmitting(true)

    try {
      await register(email, password, name)
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError("Failed to connect to server")
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="flex min-h-svh flex-col items-center justify-center gap-8 bg-muted p-8 md:p-12">
      <div className="flex w-full max-w-md flex-col gap-8">
        <div className="flex items-center gap-3 self-center text-lg font-medium">
          <div className="flex size-9 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <AirplaneTakeoff className="size-5" weight="fill" />
          </div>
          Flightlog
        </div>
        <Card>
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Create an account</CardTitle>
            <CardDescription className="text-sm">
              Start tracking your flights today
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="flex flex-col gap-5">
              {error && (
                <Alert variant="destructive">
                  <WarningCircle className="size-4" />
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              )}
              <div className="flex flex-col gap-2.5">
                <Label htmlFor="name" className="text-sm">
                  Name
                </Label>
                <Input
                  id="name"
                  type="text"
                  placeholder="John Doe"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                  disabled={isSubmitting}
                  className="h-10 text-sm"
                />
              </div>
              <div className="flex flex-col gap-2.5">
                <Label htmlFor="email" className="text-sm">
                  Email
                </Label>
                <Input
                  id="email"
                  type="email"
                  placeholder="you@example.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  disabled={isSubmitting}
                  className="h-10 text-sm"
                />
              </div>
              <div className="flex flex-col gap-2.5">
                <Label htmlFor="password" className="text-sm">
                  Password
                </Label>
                <Input
                  id="password"
                  type="password"
                  placeholder="At least 8 characters"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  minLength={8}
                  disabled={isSubmitting}
                  className="h-10 text-sm"
                />
              </div>
              <Button type="submit" disabled={isSubmitting} size="lg">
                {isSubmitting ? "Creating account..." : "Create account"}
              </Button>
              <div className="text-center text-sm text-muted-foreground">
                Already have an account?{" "}
                <Link
                  to={ROUTES.LOGIN}
                  className="text-primary underline-offset-4 hover:underline"
                >
                  Sign in
                </Link>
              </div>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
