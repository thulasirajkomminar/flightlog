# My Flights

The full logbook lives here — every flight you've ever saved, grouped by year, sorted from most recent to oldest.

![My Flights page](../assets/screenshots/04-my-flights-page.png)

## Year tabs

The row of years across the top is a quick filter. Click a year to scope the list below to that year only — handy when you want to see "how much did I fly in 2024?" without scrolling.

The currently selected year is highlighted in the brand blue; years with no flights still show in the tab strip, just dimmed.

## The flight rows

Each row is a one-line summary of a flight:

- **Airline + flight number** on the far left
- **Origin** IATA, with date and local departure time
- **Distance** in km, with a small aeroplane icon for visual orientation
- **Destination** IATA, with date and local arrival time
- **Status badge** — `Landed`, `Cancelled`, etc.
- **Chevron** to expand the row
- **Trash icon** to delete the flight

## Expanding a row

Click the chevron on the right-hand side of any row and it opens up to show the full detail — the same view you saw on the Search & Add page when you originally saved it: terminals, gates, scheduled vs runway times, aircraft type, UTC times, the lot.

This is the cached snapshot from the moment you added the flight. Flightlog **doesn't re-poll AeroDataBox** to refresh historical flights — once it's in your log, it stays exactly the way it was when you saved it. Which is usually what you want for a historical record, but worth knowing if a flight was still in the air when you added it.

## Searching and filtering

The box above the list searches your whole log — every year at once — by flight number, airline, airport code, airport name or city. Type `tokyo`, `KL`, `LHR` or `emirates` and the year tabs make way for the matching flights.

The **status** menu next to it narrows the list to landed, scheduled, in-the-air, cancelled or diverted flights. It works together with the search and with the year tabs.

## Deleting a flight

The trash icon prompts for confirmation, then removes the flight permanently. The dashboard totals update immediately to reflect the deletion.

If you delete by accident: the only "undo" is to re-search and re-add. Flightlog doesn't keep a tombstone.

!!! warning "There's no soft-delete"
    Deletion is final. If you're about to do a big cleanup and you're not 100% sure, **export your data first** ([Export](import-export.md#export)). The CSV is your backup — you can re-import it later if you change your mind.

## Import & Export buttons

The top-right of the page holds the two data-portability buttons:

- **Import** — bring flights in from a Flighty CSV or a previous Flightlog export.
- **Export** — dump your entire logbook to a CSV file.

Both are covered in detail on the [Import & Export](import-export.md) page.

## Editing a flight

Flights you [added by hand](search-and-add.md#adding-a-flight-by-hand) or imported from a CSV have a pencil button next to the delete button. It opens the same form, so you can fix a date, a flight number or an airport.

Flights that came from AeroDataBox can't be edited — that data is shared with every account on the instance. If one is wrong, delete it and add the flight by hand instead.
