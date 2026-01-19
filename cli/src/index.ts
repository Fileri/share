#!/usr/bin/env bun
import { parseArgs } from "util";
import { upload } from "./commands/upload";
import { list } from "./commands/list";
import { deleteItem } from "./commands/delete";

const { values, positionals } = parseArgs({
  args: Bun.argv.slice(2),
  options: {
    help: { type: "boolean", short: "h" },
    version: { type: "boolean", short: "v" },
    raw: { type: "boolean", short: "r" },
    type: { type: "string", short: "t" },
    server: { type: "string", short: "s" },
  },
  allowPositionals: true,
  strict: false,
});

async function main() {
  if (values.version) {
    console.log("share v0.1.0");
    process.exit(0);
  }

  if (values.help || positionals.length === 0) {
    printHelp();
    process.exit(0);
  }

  const command = positionals[0];

  switch (command) {
    case "list":
      await list();
      break;
    case "delete":
      if (positionals.length < 2) {
        console.error("Usage: share delete <id>");
        process.exit(1);
      }
      await deleteItem(positionals[1]);
      break;
    default:
      // Treat as file upload
      await upload(command, {
        raw: values.raw as boolean,
        type: values.type as string,
      });
  }
}

function printHelp() {
  console.log(`share - CLI-first file sharing service

DESCRIPTION:
  Upload files and get shareable URLs. Supports text, code, markdown,
  images, and any file type. Text/code/markdown files are rendered with
  syntax highlighting in the browser. Original files always downloadable.

COMMANDS:
  share <file>                Upload file, print URL to stdout
  share <file1> <file2> ...   Upload multiple files (not yet supported)
  share list                  List all uploads for current token
  share delete <id>           Delete upload by ID (24-char hex string)
  share info <id>             Show upload metadata (not yet supported)

STDIN:
  <command> | share           Upload piped content as text/plain
  share -                     Explicit stdin read

OPTIONS:
  -r, --raw                   Set default view to raw (no rendering)
  -t, --type <mime>           Force content-type (e.g., text/markdown)
  -s, --server <url>          Override server URL for this command
  -h, --help                  Show this help message
  -v, --version               Show version number

OUTPUT:
  upload    Prints URL only to stdout (e.g., https://v10b.no/abc123def456)
  list      Prints table: ID, SIZE, CREATED, FILENAME
  delete    Prints "Deleted: <id>" on success

EXIT CODES:
  0         Success
  1         Error (auth failure, network error, not found, etc.)

URL STRUCTURE:
  /<id>                       View with default rendering
  /<id>/raw                   Download original file
  /<id>/render                Force rendered view (markdown/code)

CONFIGURATION:
  File: ~/.config/share/config.yaml

  Required fields:
    server: <url>             Server URL (e.g., https://share.example.com)
    token: <token>            Auth token (string or op:// reference)

  Example with 1Password:
    server: https://share.tail848835.ts.net
    token: op://Development/share-server/token

  Environment variables (override config file):
    SHARE_SERVER              Server URL
    SHARE_TOKEN               Auth token

AUTHENTICATION:
  Token is sent via Authorization header. Obtain from server admin.
  Supports 1Password CLI references (op://vault/item/field).

CONTENT TYPES:
  Auto-detected from file extension. Override with --type flag.
  Rendered in browser: .md, .txt, .json, .yaml, .py, .js, .ts, .go, etc.
  Served as-is: images, PDFs, binaries, HTML

EXAMPLES:
  # Upload a markdown file
  share README.md
  # Output: https://v10b.no/a1b2c3d4e5f6g7h8

  # Upload and copy URL to clipboard (macOS)
  share notes.md | pbcopy

  # Upload code without syntax highlighting
  share --raw script.py

  # Pipe command output
  kubectl get pods | share

  # Force content type
  share --type text/markdown notes.txt

  # List your uploads
  share list

  # Delete an upload
  share delete a1b2c3d4e5f6g7h8

REPOSITORY:
  https://github.com/Fileri/share
`);
}

main().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
