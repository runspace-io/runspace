# Runspace landing page

The marketing site is a standalone Hugo project. It mounts the approved
Runspace logo assets from `apps/web/public/brand`.

## Local development

```powershell
hugo server --source . --bind 127.0.0.1 --port 1313
```

Run the command from this `landing` directory, then open
`http://127.0.0.1:1313/`.

## Production build

```powershell
hugo --source . --destination public --cleanDestinationDir --minify --panicOnWarning
```

## Cloudflare Pages

The existing `runspace` project uses direct uploads and its production branch
is `master`.

```powershell
$env:CLOUDFLARE_API_TOKEN = (Get-Content -Raw ..\cloudflare-token.txt).Trim()
$env:CLOUDFLARE_ACCOUNT_ID = '12e8ec1696f7cdef14435774b6909655'
pnpm dlx wrangler@latest pages deploy .\public --project-name=runspace --branch=master --commit-dirty=true
```

The API token must have **Cloudflare Pages: Edit** access for the account.
