import { useState } from "react"
import { toast } from "sonner"

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "~/components/ui/alert-dialog"
import { Button } from "~/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "~/components/ui/dialog"
import { Input } from "~/components/ui/input"
import { Label } from "~/components/ui/label"
import { Separator } from "~/components/ui/separator"
import { ApiError } from "~/lib/api"

import { useAuth } from "~/contexts"
import { authService } from "~/services"

const MIN_PASSWORD_LENGTH = 8

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof ApiError ? err.message : fallback
}

function ProfileForm({ onSaved }: { onSaved: () => void }) {
  const { user, updateUser } = useAuth()
  const [name, setName] = useState(user?.name ?? "")
  const [email, setEmail] = useState(user?.email ?? "")
  const [isSaving, setIsSaving] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setIsSaving(true)
    try {
      updateUser(await authService.updateProfile(email, name))
      onSaved()
      toast.success("Profile updated")
    } catch (err) {
      toast.error(errorMessage(err, "Failed to update profile"))
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <Label htmlFor="profile-name">Name</Label>
        <Input
          id="profile-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
          disabled={isSaving}
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor="profile-email">Email</Label>
        <Input
          id="profile-email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          disabled={isSaving}
        />
      </div>
      <Button type="submit" disabled={isSaving} className="self-end">
        {isSaving ? "Saving..." : "Save Changes"}
      </Button>
    </form>
  )
}

function PasswordForm() {
  const [current, setCurrent] = useState("")
  const [next, setNext] = useState("")
  const [isSaving, setIsSaving] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setIsSaving(true)
    try {
      await authService.changePassword(current, next)
      setCurrent("")
      setNext("")
      toast.success("Password changed")
    } catch (err) {
      toast.error(errorMessage(err, "Failed to change password"))
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4">
      <h3 className="text-sm font-medium">Change password</h3>
      <div className="flex flex-col gap-2">
        <Label htmlFor="profile-current-password">Current password</Label>
        <Input
          id="profile-current-password"
          type="password"
          autoComplete="current-password"
          value={current}
          onChange={(e) => setCurrent(e.target.value)}
          required
          disabled={isSaving}
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor="profile-new-password">New password</Label>
        <Input
          id="profile-new-password"
          type="password"
          autoComplete="new-password"
          minLength={MIN_PASSWORD_LENGTH}
          value={next}
          onChange={(e) => setNext(e.target.value)}
          required
          disabled={isSaving}
        />
      </div>
      <Button
        type="submit"
        variant="secondary"
        disabled={isSaving}
        className="self-end"
      >
        {isSaving ? "Changing..." : "Change Password"}
      </Button>
    </form>
  )
}

function DeleteAccount() {
  const { deleteAccount } = useAuth()
  const [password, setPassword] = useState("")
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)

  async function handleDelete() {
    setIsDeleting(true)
    try {
      await deleteAccount(password)
      toast.success("Account deleted")
    } catch (err) {
      toast.error(errorMessage(err, "Failed to delete account"))
      setIsDeleting(false)
    }
  }

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault()
        setConfirmOpen(true)
      }}
    >
      <h3 className="text-sm font-medium text-destructive">Delete account</h3>
      <p className="text-sm text-muted-foreground">
        Permanently removes your account and your flight collection. Export your
        flights first if you want to keep them.
      </p>
      <div className="flex flex-col gap-2">
        <Label htmlFor="profile-delete-password">Password</Label>
        <Input
          id="profile-delete-password"
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          disabled={isDeleting}
        />
      </div>
      <Button
        type="submit"
        variant="destructive"
        disabled={isDeleting}
        className="self-end"
      >
        {isDeleting ? "Deleting..." : "Delete Account"}
      </Button>

      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete your account?</AlertDialogTitle>
            <AlertDialogDescription>
              This cannot be undone. Your account and all flights in your
              collection will be removed.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete}>
              Delete account
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </form>
  )
}

export function ProfileDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Settings</DialogTitle>
          <DialogDescription>
            Manage your profile, password and account
          </DialogDescription>
        </DialogHeader>
        {/* Forms mount with the dialog, so they start from the current user. */}
        <ProfileForm onSaved={() => onOpenChange(false)} />
        <Separator />
        <PasswordForm />
        <Separator />
        <DeleteAccount />
      </DialogContent>
    </Dialog>
  )
}
