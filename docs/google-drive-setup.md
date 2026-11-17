# Google Drive setup

Phase 5 exports every ready photo and boomerang plus a matching message TXT file to Google Drive. SeaweedFS and MongoDB remain the primary store; Drive failures never block uploads or the showcase.

## Google Cloud

1. Create or select a Google Cloud project.
2. Enable the **Google Drive API** and **Google Picker API**.
3. Configure the OAuth consent screen.
4. Create an OAuth **Web application** client.
5. Add the backend callback as an authorized redirect URI:

```text
http://localhost:8080/api/v1/drive/oauth/callback
```

Production must use its public HTTPS backend URL.

6. Add the frontend as an authorized JavaScript origin:

```text
http://localhost:3000
```

The Picker runs in the browser, so the frontend origin must be allowed even though OAuth completes on the backend.

7. Create a browser API key restricted to:
   - Google Picker API
   - HTTP referrers:
     ```text
     http://localhost:3000/*
     ```
     plus the production frontend origin.

8. Use the Google Cloud project number as the Picker app ID.

The app uses the non-sensitive `https://www.googleapis.com/auth/drive.file` scope. Google Picker is used to select a destination folder that the app is allowed to write to.

## Backend environment

```env
PUBLIC_APP_URL=http://localhost:3000
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GOOGLE_OAUTH_REDIRECT_URL=http://localhost:8080/api/v1/drive/oauth/callback
GOOGLE_PICKER_API_KEY=
GOOGLE_PICKER_APP_ID=
DRIVE_TOKEN_ENCRYPTION_KEY=
DRIVE_SETUP_TOKEN_TTL_MINUTES=30
DRIVE_EXPORT_CONCURRENCY=1
DRIVE_QUEUE_LIMIT=1000

`GOOGLE_PICKER_APP_ID` is the Google Cloud **project number**, not the project ID.

`DRIVE_TOKEN_ENCRYPTION_KEY` must remain stable. Changing it makes existing stored OAuth refresh tokens unreadable and forces every Drive connection to reconnect.
```

Generate the token encryption key:

```bash
openssl rand -base64 32
```

Store it separately from database backups and keep a controlled recovery copy. Changing it makes existing stored OAuth refresh tokens unreadable.

Start the export worker with:

```env
WORKER_MODE=all
```

`boomerang` runs only video processing. `drive` runs only Drive export. `all` runs both.

## OAuth scope note

The customer flow uses the full Drive scope (`https://www.googleapis.com/auth/drive`) so a pasted folder link works for any folder the connected Google account can edit, including a folder shared with “anyone with the link can edit”. This is broader than `drive.file`.

Google may require OAuth app verification before this scope is used by non-test accounts. For a restricted-scope deployment, switch back to `drive.file` and require Google Picker selection instead of pasted links.

## Customer account flow

1. A signed-in customer opens one of their events.
2. Connect Google Drive from the event page.
3. The backend stores one encrypted refresh token per customer account.
4. Paste the event folder link or folder ID.
5. The backend validates write access before saving the destination.
6. New ready media exports there automatically.
7. The same connection is reused for the customer's other events; each event keeps its own destination folder.

If the customer has not selected a folder, the event uses the operator destination only when an admin has configured one. A revoked customer connection pauses exports and never silently falls back to the operator Drive.

## Operator flow

1. Open an event in admin.
2. Connect the operator Google Drive.
3. Choose the operator destination folder with Google Picker.
4. Existing ready media is queued for export.

## Customer flow

1. Admin creates a one-time customer setup link.
2. Send the link to the event customer.
3. The customer authorizes Google Drive and selects a folder with Picker.
4. The event destination switches to that customer connection.

A revoked customer connection pauses that event's exports. It never falls back silently to the operator Drive.

## Drive layout

```text
<Event Name> [<shortEventId>]/
  <Uploader Name>/
    <mediaId>.jpg
    <mediaId>.message.txt
    <mediaId>.mp4
    <mediaId>.message.txt
```

Each media item has one matching TXT file. Reusing an attendee's guest identity reuses the same uploader folder. Two attendees with the same display name keep separate folders.

## Security

- OAuth refresh tokens are encrypted with AES-GCM in MongoDB.
- No customer folder ACLs are changed and no public links are created.
- Google credentials and access tokens are never stored in the frontend.
- Picker access tokens are short-lived and returned only to the setup browser session.
- Retention and takedown delete exported media and TXT files using Drive `appProperties`.

## Verification gate

Before release, test with real operator and customer accounts:

- Folder write validation.
- Photo JPG and TXT pair.
- Boomerang MP4 and TXT pair.
- Two submissions from one attendee reuse the folder.
- Same-name attendees do not merge.
- Revoked customer connection stops exports without operator fallback.
- Retry does not create duplicate files.
- Retention removes the exported files.
