export function formatTime(dateTimeString: string): string {
  const date = new Date(dateTimeString)
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
}
