# PSAP business-owner demo

The React interface uses the PSAP API for its assets, LAUNCH questionnaires,
tiers, workers, and assessment history. It currently implements the
Business Owner role only.

## Run locally

Start the Temporal development server and the PSAP flow worker as described in
the [backend README](../backend/README.md). In another terminal, start the HTTP
API from `backend/`:

```bash
MODE_PROC=httpserver go run .
```

Then start the interface:

```bash
cd frontend
npm install
npm run dev
```

Vite serves the app on port 5173 and proxies `/api` requests to the API at
`http://localhost:8080`. To use another API base URL, set
`VITE_API_BASE_URL` to the full PSAP API root, for example
`http://localhost:8080/v1/physec/accesscontrol/psap`.

The API must be seeded with workers and business-manager assignments for asset
requests. New assets are saved with `pending-launch` state, then the interface
starts a LAUNCH workflow. If workflow startup fails, the asset request remains
saved and can be retried from the asset page.
