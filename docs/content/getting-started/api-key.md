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

## Which endpoints does Flightlog hit?

Only one of them costs quota in normal use:

| Endpoint                                                                                                                      | Used for                                                                     | Cost                             |
| ----------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------- | -------------------------------- |
| [`GET /flights/Number/{number}/{date}`](https://doc.aerodatabox.com/rapidapi.html#/operations/GetFlight_FlightOnSpecificDate) | Looking a flight up, and checking whether it has landed.                     | Counts against your quota.       |
| `GET /health/services/feeds/FlightLiveUpdates/airports`                                                                       | Learning which airports AeroDataBox tracks live. Fetched about once a day.   | Free-tier endpoint.              |
| `GET /airports/Iata/{code}`                                                                                                   | An airport that is missing from the built-in database, such as a closed one. | One call per such airport, ever. |

Everything else on the dashboard (distances, airline names, airport details) is worked out locally, from the flight data plus a built-in database of about 9,000 airports.

## How many calls will I make?

Flightlog is built to be gentle on a free plan. It **caches every lookup** in its local SQLite database and only goes back to AeroDataBox when the answer could have changed:

- Logging a new flight = **1 API call**.
- Re-opening that flight, re-running the same search, exporting, browsing = **0 API calls**.
- Searching for a flight that doesn't exist = **1 API call**, and the answer is remembered — for a week when the date is in the past, for six hours otherwise — so trying again is free.
- Searching again for a flight that hasn't landed yet = **0 API calls**, unless its date is within a few days of today. Then it is refreshed, at most once every 30 minutes, so you see current times and gates.
- A flight in your log that was added before it flew = **1 API call** after its scheduled arrival, to pick up that it landed. If AeroDataBox doesn't confirm the landing, Flightlog tries twice more (after 6 and 24 hours) and then stops. Flights between airports AeroDataBox doesn't track live are never checked at all. Set `FLIGHT_STATUS_REFRESH=false` to switch these checks off entirely.
- Re-importing your own Flightlog CSV export = **0 API calls** (everything is in the file).
- Importing a Flighty CSV = **0 API calls** with enrichment off; with it on, up to 1 call per flight under one year old.

If you're worried about quota, leave enrichment off when you import — you'll still get the basics (date, airline, route) from the CSV itself.

Flightlog also watches the quota AeroDataBox reports. When less than a fifth of it is left, the automatic landing checks pause so the rest is kept for your own searches. If the quota runs out completely, Flightlog stops calling for a while and tells you so, instead of failing slowly.

!!! info "Keys are local"
    Your API key only ever leaves the Flightlog container in outbound requests to AeroDataBox. It's never sent to your browser, never logged in plain text, never shared with anything else.
