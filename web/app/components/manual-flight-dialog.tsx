import { useState } from "react"
import { toast } from "sonner"

import { Button } from "~/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "~/components/ui/dialog"
import { Input } from "~/components/ui/input"
import { Label } from "~/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select"
import { ApiError } from "~/lib/api"

import type { Flight, ManualFlightInput } from "~/types"

import { flightService } from "~/services"

const STATUSES: Record<string, string> = {
  landed: "Landed",
  scheduled: "Scheduled",
  cancelled: "Cancelled",
  diverted: "Diverted",
}

function initialInput(flight?: Flight): ManualFlightInput {
  return {
    number: flight?.number.replace(/\s+/g, "") ?? "",
    date: flight?.flightDate ?? "",
    departureIata: flight?.departure.airport.iata ?? "",
    arrivalIata: flight?.arrival.airport.iata ?? "",
    airlineName: flight?.airline.name ?? "",
    aircraftModel: flight?.aircraft.model ?? "",
    status: flight && flight.status in STATUSES ? flight.status : "landed",
  }
}

interface ManualFlightFormProps {
  flight?: Flight
  onDone: () => void
}

// The form mounts with the dialog, so it always starts from the current flight.
function ManualFlightForm({ flight, onDone }: ManualFlightFormProps) {
  const [input, setInput] = useState(() => initialInput(flight))
  const [isSaving, setIsSaving] = useState(false)

  function set(field: keyof ManualFlightInput, value: string) {
    setInput((current) => ({ ...current, [field]: value }))
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setIsSaving(true)
    try {
      if (flight) {
        await flightService.updateManual(flight.id, input)
        toast.success("Flight updated")
      } else {
        await flightService.createManual(input)
        toast.success("Flight added")
      }
      onDone()
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to save the flight"
      )
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-4">
        <div className="flex flex-col gap-2">
          <Label htmlFor="manual-number">Flight number</Label>
          <Input
            id="manual-number"
            placeholder="e.g. KL641"
            value={input.number}
            onChange={(e) => set("number", e.target.value)}
            required
            disabled={isSaving}
          />
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="manual-date">Date</Label>
          <Input
            id="manual-date"
            type="date"
            value={input.date}
            onChange={(e) => set("date", e.target.value)}
            required
            disabled={isSaving}
            className="[&::-webkit-calendar-picker-indicator]:invert"
          />
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="manual-from">From (airport code)</Label>
          <Input
            id="manual-from"
            placeholder="e.g. AMS"
            maxLength={3}
            value={input.departureIata}
            onChange={(e) => set("departureIata", e.target.value.toUpperCase())}
            required
            disabled={isSaving}
          />
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="manual-to">To (airport code)</Label>
          <Input
            id="manual-to"
            placeholder="e.g. JFK"
            maxLength={3}
            value={input.arrivalIata}
            onChange={(e) => set("arrivalIata", e.target.value.toUpperCase())}
            required
            disabled={isSaving}
          />
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="manual-airline">Airline (optional)</Label>
          <Input
            id="manual-airline"
            value={input.airlineName}
            onChange={(e) => set("airlineName", e.target.value)}
            disabled={isSaving}
          />
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="manual-aircraft">Aircraft (optional)</Label>
          <Input
            id="manual-aircraft"
            value={input.aircraftModel}
            onChange={(e) => set("aircraftModel", e.target.value)}
            disabled={isSaving}
          />
        </div>
      </div>
      <div className="flex flex-col gap-2">
        <Label>Status</Label>
        <Select
          value={input.status}
          onValueChange={(v) => set("status", v)}
          disabled={isSaving}
        >
          <SelectTrigger className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {Object.entries(STATUSES).map(([value, label]) => (
              <SelectItem key={value} value={value}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <DialogFooter>
        <Button type="submit" disabled={isSaving}>
          {isSaving ? "Saving..." : flight ? "Save Changes" : "Add Flight"}
        </Button>
      </DialogFooter>
    </form>
  )
}

interface ManualFlightDialogProps {
  // The flight to change; omit to add a new one.
  flight?: Flight
  trigger: React.ReactNode
  onSaved?: () => void
}

// Adds a flight by hand, or edits one the user imported or added earlier.
// It is the way in for flights the data provider does not know.
export function ManualFlightDialog({
  flight,
  trigger,
  onSaved,
}: ManualFlightDialogProps) {
  const [open, setOpen] = useState(false)

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {flight ? "Edit flight" : "Add a flight by hand"}
          </DialogTitle>
          <DialogDescription>
            {flight
              ? "Change the details of this flight."
              : "For flights the search does not find, such as older ones. Distance is worked out from the two airports."}
          </DialogDescription>
        </DialogHeader>
        <ManualFlightForm
          flight={flight}
          onDone={() => {
            setOpen(false)
            onSaved?.()
          }}
        />
      </DialogContent>
    </Dialog>
  )
}
