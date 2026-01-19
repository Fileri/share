# share

CLI-first file sharing. Upload files, get URL.

```bash
share file.md
# https://your-domain.com/x8k2m4pqz9n3wvb7

cat notes.txt | share
# https://your-domain.com/a9b3c7d2e5f8g1h4
```

## Features

- **CLI-first**: No web UI for upload, just `share <file>` or pipe stdin
- **Universal content**: Files, markdown, code, HTML pages, images
- **Smart rendering**: Markdown and code render in browser (client-side)
- **Unlisted links**: 16+ char random IDs, not indexed by search engines
- **Self-hosted**: Your server, your domain, your data

## Installation

### CLI

```bash
# bun
bun install -g @fileri/share

# or download binary
curl -fsSL https://your-domain.com/install.sh | sh
```

### Server

```bash
docker pull ghcr.io/fileri/share:latest

docker run -d \
  -p 8080:8080 \
  -v /path/to/config.yaml:/etc/share/config.yaml \
  ghcr.io/fileri/share
```

## Usage

```bash
# Upload a file
share document.pdf

# Pipe content
echo "Hello world" | share
cat script.py | share

# Force content type
share --type html page.txt

# Upload as raw (no rendering)
share --raw notes.md

# List your uploads
share list

# Delete an upload
share delete x8k2m4pqz9n3wvb7
```

## URL Paths

| Path | Description |
|------|-------------|
| `/<id>` | Default view (uploader's preference) |
| `/<id>/raw` | Original file |
| `/<id>/render` | Force rendered view |

## Configuration

### CLI Config

`~/.config/share/config.yaml`:

```yaml
server: https://your-domain.com
token: your-api-token
# or use 1Password
# token: op://Vault/share/token
```

### Server Config

```yaml
domain: your-domain.com
base_url: https://your-domain.com

storage:
  type: s3
  endpoint: storage.googleapis.com
  bucket: share-files
  region: us-central1

limits:
  max_file_size: 100MB  # 0 = unlimited
  rate_limit: 0         # 0 = unlimited
```

## Stack

| Component | Technology |
|-----------|------------|
| CLI | TypeScript / Bun |
| Server | Go |
| Storage | S3-compatible |
| Rendering | marked.js, highlight.js |

## License

MIT
