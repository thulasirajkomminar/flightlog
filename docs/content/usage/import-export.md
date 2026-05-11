# Import & Export

Flightlog treats your logbook as **your data**. There's a one-click export of everything in a portable CSV, and an import that can either ingest your own export back, or pull in a CSV from another tracker.

Both live in the top-right of the [My Flights](my-flights.md) page.

## Export

Click **Export** on the My Flights page and your browser downloads a file called `flightlog-export.csv`.

The file is a complete dump — 29 columns covering every field Flightlog knows about. Roughly:

- **Identity** — flight number, airline code, airline name
- **Route** — origin / destination IATA, ICAO, city, country, latitude/longitude
- **Times** — scheduled departure / arrival, revised, runway, all in both local and UTC
- **Logistics** — terminal, gate, aircraft type/registration
- **Computed** — distance in km

If you can open it in a spreadsheet, you can do anything with it. Filter, pivot, hand-edit, feed it into another system, archive it.

!!! tip "Use exports as your backup"
    Even if you back up the SQLite volume, keeping a fresh `flightlog-export.csv` around (in your usual document backup, in a Git repo, in iCloud Drive — wherever) is a nice belt-and-braces approach. The format is stable and the file is small.

## Import

Click **Import** on My Flights to open the import dialog. The flow has four steps:

### 1. Select source

Pick the source format from the dropdown. Two are supported:

| Source        | Use it when                                                                                                     |
| ------------- | --------------------------------------------------------------------------------------------------------------- |
| **Flightlog** | You're re-importing your own `flightlog-export.csv` (e.g. moving to a new instance, or restoring after a wipe). |
| **Flighty**   | You're migrating from the [Flighty](https://www.flightyapp.com) app and have its CSV export.                    |

### 2. Upload CSV

Pick the file. Two soft limits to be aware of:

- **5 MB** maximum file size
- **100 flights** per import — split larger files into batches

The dialog parses the CSV right away and tells you how many flights it found before you commit.

### 3. (Flighty only) Enrich

A Flighty CSV is intentionally minimal — basically date, airline, flight number, origin, destination. Useful, but missing things like terminal, gate, aircraft type and actual times.

If you tick **Enrich**, Flightlog will call AeroDataBox for each flight that's within the last twelve months and backfill the missing fields. Flights older than a year are imported with what's in the CSV (AeroDataBox doesn't have historical data for them anyway).

- **Enrich ON** = one API call per recent flight, prettier results.
- **Enrich OFF** = zero API calls, basic data only.

For a Flightlog → Flightlog import this step doesn't exist — your own export already has everything.

### 4. Import

Hit the button, watch the count tick up. The dialog confirms when it's done, and the new flights show up in My Flights and on the Dashboard right away.

## Migration recipes

### Moving to a new host

1. On the old instance: **Export** → save the CSV somewhere.
2. Spin up Flightlog on the new host.
3. Create your account.
4. **Import** → source = **Flightlog** → upload the CSV.

Done. Zero API calls, all your data, including airline names and coordinates, lands intact.

### Coming from Flighty

1. In Flighty, **Settings → Export Flights** → save the CSV.
2. In Flightlog, **Import** → source = **Flighty** → upload.
3. Decide on enrichment based on how recent your flights are and how much you care about the missing fields.
4. Import.

### Bulk-editing your data

1. **Export.**
2. Open the CSV in your spreadsheet of choice and edit.
3. _(Optional)_ delete everything in your Flightlog logbook to avoid duplicates.
4. **Import** as **Flightlog** source.

The CSV is the source of truth — Flightlog will faithfully recreate whatever's in it.
