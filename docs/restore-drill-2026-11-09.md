# Local restore drill — 2026-11-09

Environment: local Docker dev stack.

Backup: `dev-20261109T064805Z`

## Backup contents

- MongoDB archive: 14 KB
- SeaweedFS archive: 10 MB
- Collections: events, media_items, media_jobs, customers, admin_sessions, templates,
  template_versions, assets, event_plans, drive_connections, drive_destinations,
  drive_export_jobs, drive_guests, platform_settings

## Restore result

- MongoDB restored: 91 documents, 0 failures
- SeaweedFS volume restored
- Event count after restore: 5
- Media item count after restore: 22
- Template count after restore: 1

## Post-restore verification

- `/healthz`: live
- `/readyz`: ready
- Admin login: success
- Public event: `custom-smoke-9383b2`
- Media fetch after restore: `200`, 26,007 bytes, `photo`
- Display and billing data present in MongoDB

## Notes

- This drill used local dev credentials and Docker volumes only.
- It did not exercise off-host storage, network latency or production credentials.
- Existing browser sessions were invalid after restore; log in again.
- Restore took well under a minute locally.
