# Oshimai Record & Replay (Chrome Extension)

Records the network requests a browser tab makes while you click through a real user journey, then exports them as a `.har` file or sends them straight to a running Oshimai server — turning a five-minute manual walkthrough into a load-test scenario, with no OpenAPI spec required.

## Install (unpacked, for now)

1. Open `chrome://extensions`.
2. Enable **Developer mode** (top right).
3. Click **Load unpacked** and select this `tools/browser-extension` directory.

## Use

1. Click the Oshimai icon, then **Start Recording**.
2. Use the target web app normally — sign in, browse, checkout, whatever the journey is.
3. Click **Stop**, then either **Export HAR** (downloads a `.har` file you can import anywhere) or enter your Oshimai server URL and click **Send to Oshimai** to generate a scenario immediately via `POST /api/v1/scenarios/import/har`.

## Files

- `manifest.json` — Manifest V3 config
- `background.js` — service worker capturing requests via `chrome.webRequest` into HAR-entry shape
- `popup.html` / `popup.js` — the toolbar popup UI (start/stop, export, send)
