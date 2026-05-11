# Getting an AeroDataBox API key

Flightlog needs one piece of help from the outside world: when you punch in a flight number, _something_ has to know that **QR 8501 on 13 Oct 2026** went from Auckland to Honolulu on a Boeing 777, departed from gate F3, and landed at gate A58. That something is [AeroDataBox](https://aerodatabox.com).

AeroDataBox publishes its API through [RapidAPI](https://rapidapi.com/aedbx-aedbx/api/aerodatabox), which is where you'll grab a key.

## Step-by-step

1. **Create a RapidAPI account.** Head to [rapidapi.com](https://rapidapi.com) and sign up — Google or GitHub sign-in works fine.

2. **Subscribe to AeroDataBox.** Go to the [AeroDataBox listing on RapidAPI](https://rapidapi.com/aedbx-aedbx/api/aerodatabox) and click **Subscribe to Test**. Pick the **BASIC** plan — it's free and gives you a healthy monthly quota that's more than enough for personal use.

3. **Copy your `X-RapidAPI-Key`.** On the AeroDataBox page, look at the right-hand panel under **Header Parameters**. You'll see a key labelled `X-RapidAPI-Key` with a long string. That's your API key — copy it.

4. **Paste it into your `.env`.**

   ```bash title=".env"
   AERODATABOX_API_KEY=your_long_key_string_here
   ```

That's the whole setup. Restart the container after editing `.env` and you're done.

## Which endpoint does Flightlog hit?

Just one: [`GET /flights/Number/{number}/{date}`](https://doc.aerodatabox.com/rapidapi.html#/operations/GetFlight_FlightOnSpecificDate) — _Flight on a specific date_. Every other piece of info on the dashboard (distances, airline names, airport metadata) is derived locally from the data this endpoint returns plus a small built-in airport database.

## How many calls will I make?

Flightlog **caches every lookup** in its local SQLite database. The same flight number + date combination is fetched from AeroDataBox once and then served from the cache forever after. In practice:

- Logging a new flight = **1 API call**
- Re-opening that flight, re-running the same search, exporting, browsing = **0 API calls**
- Re-importing your own Flightlog CSV export = **0 API calls** (everything is in the file)
- Importing a Flighty CSV with enrichment turned on = up to 1 call per flight under one year old

If you're worried about quota, leave enrichment off when you import — you'll still get the basics (date, airline, route) from the CSV itself.

!!! info "Keys are local"
    Your API key only ever leaves the Flightlog container in outbound requests to AeroDataBox. It's never sent to your browser, never logged in plain text, never shared with anything else.
