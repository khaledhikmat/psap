# PSAP business portals demo

The shared React interface uses the PSAP API for assets, questionnaires, tiers,
workers, and assessment history. The demo user picker selects assigned Business
Owner and Business Manager actors and routes to `/business-owner` or
`/business-manager`. Both roles can read historic LAUNCH and SHIELD answers.

Business Owners can request assets, complete LAUNCH answers, and request
reassessments. Business Managers can review submitted LAUNCH answers, accept the
calculated tier or assign a tier override, and start or continue SHIELD
assessments with required evidence. Manager SHIELD answers and evidence are
saved through the API. New LAUNCH tier approvals remain open for seven days
after the tier is calculated. Timed-out LAUNCH records remain visible in the
manager queue with guidance to request a reassessment. Security Assessor and
Implementer portals are not part of this demo.

The asset portfolio is a locally searchable table, so names, IDs, locations,
tiers, and statuses can be filtered without another API request. This is demo
role selection, not authentication or authorization.

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
