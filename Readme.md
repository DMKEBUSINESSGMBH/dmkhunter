# DMKhunter

**This package is experimental, the configuration and usage may still change.**

DMKhunter detects new and changed files in your web projects. It stores an MD5
hash of every file in a database and reports every file that was added or
modified since then, by printing a table, sending a mail or calling a webhook.

It is meant for hosting environments where you can't install system tools. If
you can install packages, use `rkhunter` and `clamav` as well.

## How it works

1. `rescan` hashes all files in the configured directories and saves the hashes
   as the known good state.
2. `analyze` hashes all files again and compares them with the saved state:
   - files that are not in the database are reported as `New file detected`
   - files whose hash differs are reported as `The file … has been changed!`
   - files that can't be read are reported as warnings
3. After a deployment or any other intended change, run `rescan` again so the
   new state becomes the known good one.

Deleted files are not reported.

## Installation

```sh
go build -o dmkhunter .
```

Copy the binary to the server and create a `.hunter.conf` next to it, see
[Configuration](#configuration).

## Usage

```
dmkhunter [-config <file>] [-v|-vv|-vvv] <command>
```

| Command   | Description                                                           |
|-----------|-----------------------------------------------------------------------|
| `rescan`  | hash all files and save them as the known good state                 |
| `analyze` | compare all files with the saved state and report differences        |
| `list`    | show the saved hashes as a table                                     |

Global flags can be given at any position, `dmkhunter analyze -v` works as well
as `dmkhunter -v analyze`.

| Flag             | Description                                                     |
|------------------|-----------------------------------------------------------------|
| `-config <file>` | configuration file, default `.hunter.conf` in the current directory |
| `-v`             | show a spinner while working                                    |
| `-vv`            | show the spinner, the number of files and the current file      |
| `-vvv`           | like `-vv` plus hashes per second, and the runtime when done    |

Without a verbosity flag only errors and the analyze report are printed, which
is what you want for cron jobs. The progress line goes to stderr.

### First run

```sh
dmkhunter rescan -v
dmkhunter analyze -v
```

If the database already exists, `rescan` asks whether it should be deleted
first. Answer `y` to start from scratch. Any other answer, or no terminal at
all, keeps the database and only adds and updates hashes, so entries of files
that were deleted in the meantime stay in it.

### Regular checks

Run `analyze` from cron. Reporters are only called when something was found:

```cron
*/30 * * * * cd /path/to/dmkhunter && ./dmkhunter analyze
```

The database and configuration paths are relative to the current directory,
so change into the directory first or use absolute paths.

### Exit codes

| Code | Meaning                                         |
|------|-------------------------------------------------|
| 0    | success, `analyze` found nothing                |
| 1    | `analyze` found new or changed files            |
| 2    | error, e.g. missing configuration or database, or a mail or webhook could not be sent |

### Listing the database

```sh
dmkhunter list                                  # all records
dmkhunter list -prefix /var/www/shop/src        # paths starting with the prefix
dmkhunter list -contains config -limit 20       # paths containing "config", at most 20
dmkhunter list -db other.db                     # another database than configured
```

When several databases are listed, the table gets a `Database` column.

## Configuration

The configuration is a TOML file, see [.hunter.conf.example](.hunter.conf.example).

```toml
[[Presets]]
paths = ['/var/www/shop']
database = "hunter.db"
ignores = [
  '^var/.*\.(log|lock|cache)$',
  '(?i)^public/uploads/.*\.(jpe?g|png|gif|webp|pdf)$',
]

[smtp]
username = "smtpuser"
password = "smtppassword"
host = "smtp.example.com:587"
recipients = ["alert@example.com"]
from-address = "dmkhunter@example.com"
topic = "shop server"

[webhook]
url = "https://example.com/hooks/dmkhunter"
```

### Presets

You can add several `[[Presets]]` blocks, each with its own directories,
ignores and database.

| Key        | Description                                                       |
|------------|-------------------------------------------------------------------|
| `paths`    | directories to watch, scanned recursively                         |
| `database` | file the hashes are stored in. Always set it: `rescan` falls back to `hunter.db`, but `analyze` skips presets without a database |
| `ignores`  | regular expressions of files to skip, see below                   |
| `clamav`   | unused                                                            |

Symlinks and other non-regular files are skipped.

### Ignore patterns

Ignore patterns are [Go regular expressions](https://pkg.go.dev/regexp/syntax)
and are matched against the path **relative to the preset path**, with `/` as
separator. For the preset path `/var/www/shop` the file
`/var/www/shop/var/log/dev.log` is matched as `var/log/dev.log`. Use `^` to
anchor a pattern at the start of the project, otherwise it matches anywhere in
the path.

Ignore only what changes on its own and can't be executed. An attacker who
can write files usually drops PHP into exactly the directories that are
ignored, so don't ignore whole directories, list the harmless extensions:

```toml
ignores = [
  # bad: hides shell.php in the upload directory
  # '^public/uploads/',

  # good: only images and documents are ignored, PHP is still reported
  '(?i)^public/uploads/.*\.(jpe?g|png|gif|webp|pdf)$',
]
```

Go regular expressions have no lookahead, so "everything except .php" can't
be written directly, list the allowed extensions instead.

If the preset path contains several projects, `^([^/]+/)?` allows one project
directory in front, so the same pattern works for one and for many projects:

```toml
paths = ['/var/www']   # /var/www/shop, /var/www/blog, ...
ignores = [
  # logs, locks and cache metadata in var/
  '^([^/]+/)?var/.*\.(log|lock|meta|cache)$',
  # cache, session and rate limiter files without extension in var/
  '^([^/]+/)?var/(.*/)?[^/.]+$',
  # TYPO3 code cache, it contains generated PHP that changes on every cache flush
  '^([^/]+/)?((public|web|htdocs)/)?(typo3temp/)?var/cache/(code|data)/',
]
```

Generated PHP like the TYPO3 or Symfony cache is a trade-off: if you check it,
every cache flush is reported until the next `rescan`. If you ignore it, a
manipulated cache file goes unnoticed.

Invalid patterns abort `rescan` and `analyze` with exit code 2.

### Reporters

The report is always printed to stdout as a table. Additionally:

**SMTP** (`[smtp]`) sends one mail to all `recipients`, authenticated with
CRAM-MD5. The subject is `Alert from <topic> (<n> changes)`, `topic` defaults
to the host name and `from-address` to `dmkhunter@<topic>`.

**Webhook** (`[webhook]`) posts the violations as JSON. Any status other than
2xx is reported as an error:

```json
{
  "count": 1,
  "violations": [
    {
      "severity": "error",
      "message": "New file detected: shell.php",
      "filepath": "/var/www/shop/public/uploads/shell.php"
    }
  ]
}
```

## Development

```sh
go test ./...
go test -race ./...
make coverage
```
