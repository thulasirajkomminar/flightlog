---
hide:
  - navigation
  - toc
---

<div class="flightlog-hero" markdown>
![Flightlog logo](assets/logo.png)

# Flightlog

A small, self-hosted logbook for the flights you've actually taken.

</div>

Flightlog is for travellers who want to **keep a personal history of where they've flown** without feeding it to a third-party tracker. Punch in a flight number and date, and it pulls the route, airline, aircraft, terminals, gates and times from [AeroDataBox](https://aerodatabox.com), then stores everything locally in SQLite.

You get a world map of your trips, totals you can show off (distance, hours in the air, airports, airlines), and a tidy list you can search, export and re-import whenever you want.

![Flightlog dashboard](assets/screenshots/02-dashboard-page.png)

## What you can do with it

<div class="grid cards" markdown>

-   :material-magnify:{ .lg .middle } **Look up a flight**

    ---

    Type the flight number from your boarding pass and the date — Flightlog fetches the full schedule, route, gates and aircraft type.

    [:octicons-arrow-right-24: Search & add](usage/search-and-add.md)

-   :material-map-outline:{ .lg .middle } **See your flights on a map**

    ---

    Every flight you save shows up on the dashboard map, with passport-style totals underneath.

    [:octicons-arrow-right-24: Dashboard tour](usage/dashboard.md)

-   :material-format-list-bulleted:{ .lg .middle } **Manage your logbook**

    ---

    Browse your flights by year, expand a row for the full detail, or remove the ones that don't belong.

    [:octicons-arrow-right-24: My flights](usage/my-flights.md)

-   :material-swap-horizontal:{ .lg .middle } **Bring your data with you**

    ---

    Import a [Flighty](https://www.flightyapp.com) CSV to get started, or export your own data any time you like.

    [:octicons-arrow-right-24: Import & export](usage/import-export.md)

</div>

## Get up and running

The fastest way to try Flightlog is the published Docker image — it takes two files and one `docker compose up`.

<div class="flightlog-cta" markdown>
[:material-rocket-launch: &nbsp; Quickstart](getting-started/index.md){ .md-button .md-button--primary }
[:fontawesome-brands-github: &nbsp; View on GitHub](https://github.com/thulasirajkomminar/flightlog){ .md-button }
</div>

---

!!! note "Why self-hosted?"
    Your flight history is surprisingly personal — it's a map of where you've been and when. Flightlog runs on your own hardware (a tiny VPS, a homelab, a Raspberry Pi), keeps the database next to it, and never phones home. The only outbound call is to AeroDataBox to look up flight details — and even that's cached so the same flight is only ever fetched once.
