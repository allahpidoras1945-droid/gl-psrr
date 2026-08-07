# Glukoza

Concurrent B2B lead scraper, heuristic CIS filter, Telegram username validator, and multi-format exporter.

## Features

- Concurrent worker pool with cancellation and per-target error handling.
- Scraper registry: `affpaying.com` uses the CPA scraper; other domains use the default Colly scraper.
- Contact extraction for email, Telegram, Skype, LinkedIn, Discord, and Twitter/X.
- Hidden CPA contacts from HTML attributes plus affiliate manager and network metadata parsing.
- CIS/Slavic heuristics with audit reasons and thread-safe deduplication.
- Telegram MTProto validation with persistent sessions, interactive auth, pacing, FloodWait handling, and deleted-account detection.
- CSV, JSON, and styled XLSX exports selected by output extension.

## Requirements

- Go 1.25 or newer.
- Telegram API credentials only for live Telegram validation. Create them at `my.telegram.org`.

## Run Main Pipeline

Create a newline-delimited input file. Empty lines and lines beginning with `#` are ignored.

```powershell
go run ./cmd/app `
	-input urls.txt `
	-output output/leads.xlsx `
	-workers 20 `
	-session data/tg_session.json
```

Available flags:

```text
-input     URL input file, default: urls.txt
-output    .csv, .json, or .xlsx output path
-workers   concurrent worker count, default: 20
-tg-appid  Telegram API ID, optional
-tg-apphash Telegram API hash, optional
-session   persistent Telegram session path
```

When `-tg-appid` and `-tg-apphash` are supplied, the application starts interactive Telegram authentication on the first run. It asks for the phone number, login code, and 2FA password when needed. The session is then reused.

Without Telegram credentials, validation is disabled and results are marked `SKIPPED`.

## Telegram Diagnostic Tool

Validate handles directly without scraping a website:

```powershell
$env:TG_APP_ID = "your_app_id"
$env:TG_APP_HASH = "your_app_hash"

go run ./cmd/tgcheck `
	-usernames "@adsterra,@adverten,@biosourcenutra" `
	-session data/tg_session.json
```

Alternatively, put one handle per line in a file:

```powershell
go run ./cmd/tgcheck -file usernames.txt -session data/tg_session.json
```

The diagnostic output includes `VALID`, `NOT_FOUND`, `INVALID`, `DELETED`, user ID, bot status, and verification status.

## Tests and Checks

```powershell
go test ./...
go vet ./...
```

## Security

Never commit Telegram API hashes, API credentials, phone numbers, or session files. Local sessions under `data/`, input URL files, generated exports, and temporary Excel files are ignored by `.gitignore`.
