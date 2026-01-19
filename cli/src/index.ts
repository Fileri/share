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
  console.log(`share - CLI-first file sharing

USAGE:
  share <file>              Upload a file
  cat <file> | share        Upload from stdin
  share list                List your uploads
  share delete <id>         Delete an upload

OPTIONS:
  -r, --raw                 Default to raw view (no rendering)
  -t, --type <type>         Force content type
  -s, --server <url>        Override server URL
  -h, --help                Show this help
  -v, --version             Show version

EXAMPLES:
  share notes.md            Upload and get URL
  echo "hello" | share      Upload from stdin
  share --raw script.py     Upload without rendering
  share list                Show all your uploads
  share delete abc123       Delete an upload

CONFIGURATION:
  ~/.config/share/config.yaml

    server: https://your-domain.com
    token: your-api-token
`);
}

main().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
