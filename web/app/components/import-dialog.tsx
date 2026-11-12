import { useEffect, useRef, useState } from "react"
import { AirplaneTilt, TrayArrowDown } from "@phosphor-icons/react"
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
import { Switch } from "~/components/ui/switch"

import type { ImportJob, ImportPreview, ImportResult } from "~/types"

import { importService } from "~/services"

const SOURCES: Record<string, string> = {
  flighty: "Flighty",
  flightlog: "Flightlog",
}

function announceResult(result: ImportResult) {
  const parts = []
  if (result.imported > 0) parts.push(`${result.imported} imported`)
  if (result.skipped > 0) parts.push(`${result.skipped} skipped`)
  if (result.failed > 0) parts.push(`${result.failed} failed`)

  if (result.failed > 0) {
    toast.warning(`Import completed: ${parts.join(", ")}`)
  } else {
    toast.success(`Import completed: ${parts.join(", ")}`)
  }
}

const MAX_IMPORT_FLIGHTS = 2000
const POLL_INTERVAL_MS = 1000

interface ImportDialogProps {
  onImported: () => void
}

export function ImportDialog({ onImported }: ImportDialogProps) {
  const [open, setOpen] = useState(false)
  const [source, setSource] = useState<string>("")
  const [file, setFile] = useState<File | null>(null)
  const [preview, setPreview] = useState<ImportPreview | null>(null)
  const [enrich, setEnrich] = useState(true)
  const [isLoading, setIsLoading] = useState(false)
  const [isStarting, setIsStarting] = useState(false)
  // The import runs on the server; `job` mirrors its progress while it runs.
  const [job, setJob] = useState<ImportJob | null>(null)
  // Bumped when this dialog starts an import, to (re)start polling.
  const [startedCount, setStartedCount] = useState(0)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const isImporting = isStarting || job !== null

  function clearFile() {
    setFile(null)
    setPreview(null)
    if (fileInputRef.current) {
      fileInputRef.current.value = ""
    }
  }

  function reset() {
    clearFile()
    setSource("")
    setEnrich(true)
    setIsLoading(false)
  }

  // Latest callback, read when an import finishes, so a changing callback
  // identity never restarts polling (which would announce the result again).
  const onImportedRef = useRef(onImported)
  useEffect(() => {
    onImportedRef.current = onImported
  }, [onImported])

  // Follow the server-side import until it finishes. On first render this
  // also picks up an import that was started before the page was (re)loaded;
  // an import that had already finished by then is not announced again.
  useEffect(() => {
    let cancelled = false
    let timer: ReturnType<typeof setTimeout> | undefined
    let sawRunning = startedCount > 0

    async function poll() {
      let current: ImportJob | null
      try {
        current = await importService.status()
      } catch {
        if (!cancelled && sawRunning) {
          timer = setTimeout(poll, POLL_INTERVAL_MS)
        }
        return
      }
      if (cancelled) return

      if (current?.state === "running") {
        sawRunning = true
        setJob(current)
        timer = setTimeout(poll, POLL_INTERVAL_MS)
        return
      }

      setJob(null)
      if (sawRunning && current?.result) {
        announceResult(current.result)
        onImportedRef.current()
        setOpen(false)
        setFile(null)
        setPreview(null)
        setSource("")
      }
    }

    poll()

    return () => {
      cancelled = true
      clearTimeout(timer)
    }
  }, [startedCount])

  async function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const selected = e.target.files?.[0]
    if (!selected) return

    setFile(selected)
    setIsLoading(true)

    try {
      const result = await importService.preview(source, selected)
      if (result.total > MAX_IMPORT_FLIGHTS) {
        toast.error(
          `Too many flights: ${result.total} (max ${MAX_IMPORT_FLIGHTS})`
        )
        clearFile()
        return
      }
      setPreview(result)
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to validate file"
      )
      clearFile()
    } finally {
      setIsLoading(false)
    }
  }

  async function handleImport() {
    if (!file) return

    setIsStarting(true)

    try {
      const shouldEnrich = source !== "flightlog" && enrich
      setJob(await importService.importFlights(source, file, shouldEnrich))
      setStartedCount((n) => n + 1)
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to import flights"
      )
    } finally {
      setIsStarting(false)
    }
  }

  const progressLabel = job
    ? `Importing ${job.processed} / ${job.total}...`
    : "Importing..."

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        setOpen(v)
        // Closing the dialog does not stop a running import.
        if (!v && !isImporting) reset()
      }}
    >
      <DialogTrigger asChild>
        <Button size="lg">
          <TrayArrowDown className="size-4" />
          {isImporting ? progressLabel : "Import"}
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Import flights</DialogTitle>
          <DialogDescription>
            Import flights from a CSV export.
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label>Source</Label>
            <Select
              value={source}
              onValueChange={(v) => {
                setSource(v)
                clearFile()
              }}
              disabled={isLoading || isImporting}
            >
              <SelectTrigger className="w-full">
                <SelectValue placeholder="Select source" />
              </SelectTrigger>
              <SelectContent>
                {Object.entries(SOURCES).map(([value, label]) => (
                  <SelectItem key={value} value={value}>
                    {label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="import-file">CSV file</Label>
            <Input
              id="import-file"
              ref={fileInputRef}
              type="file"
              accept=".csv"
              className={`file:mr-3 file:font-medium file:text-foreground ${!file ? "text-muted-foreground" : ""}`}
              onChange={handleFileChange}
              disabled={!source || isLoading || isImporting}
            />
          </div>

          {job && (
            <div className="flex flex-col gap-2">
              <div
                className="h-2 w-full overflow-hidden rounded-full bg-muted"
                role="progressbar"
                aria-label="Import progress"
                aria-valuemin={0}
                aria-valuemax={job.total}
                aria-valuenow={job.processed}
              >
                <div
                  className="h-full bg-primary transition-[width]"
                  style={{
                    width: `${job.total > 0 ? (job.processed / job.total) * 100 : 0}%`,
                  }}
                />
              </div>
              <p className="text-xs text-muted-foreground">
                The import keeps running if you close this dialog.
              </p>
            </div>
          )}

          {isLoading && (
            <p className="text-sm text-muted-foreground">Validating file...</p>
          )}

          {preview && (
            <div className="flex flex-col gap-4">
              <div className="rounded-md border p-3">
                <div className="flex items-center gap-2">
                  <AirplaneTilt className="size-5 text-primary" />
                  <span className="text-sm font-medium">
                    {preview.total} flights found
                  </span>
                </div>
              </div>

              {source !== "flightlog" && preview.enrichable > 0 && (
                <div className="flex items-center justify-between gap-3">
                  <Label
                    htmlFor="enrich-toggle"
                    className="flex flex-col gap-1"
                  >
                    <span>Enrich recent flights</span>
                    <span className="text-xs font-normal text-muted-foreground">
                      {preview.enrichable} flight
                      {preview.enrichable !== 1 ? "s" : ""} within the last year
                      can be enriched with detailed data (gates, aircraft,
                      actual times)
                    </span>
                  </Label>
                  <Switch
                    id="enrich-toggle"
                    checked={enrich}
                    onCheckedChange={setEnrich}
                  />
                </div>
              )}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button onClick={handleImport} disabled={!preview || isImporting}>
            {isImporting
              ? progressLabel
              : `Import ${preview?.total ?? 0} flights`}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
