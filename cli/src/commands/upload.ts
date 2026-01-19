import { existsSync, statSync } from "fs";
import { basename, extname } from "path";
import { loadConfig } from "../config";

interface UploadOptions {
  raw?: boolean;
  type?: string;
}

export async function upload(
  fileOrStdin: string,
  options: UploadOptions
): Promise<void> {
  const config = await loadConfig();

  if (!config.token) {
    console.error("No token configured. Set SHARE_TOKEN or configure ~/.config/share/config.yaml");
    process.exit(1);
  }

  let body: BodyInit;
  let filename = "";
  let contentType = options.type || "";

  // Check if it's a file path or stdin indicator
  const isFile = existsSync(fileOrStdin);

  if (isFile) {
    // Upload file
    const file = Bun.file(fileOrStdin);
    body = file;
    filename = basename(fileOrStdin);
    contentType = contentType || detectContentType(filename);
  } else if (fileOrStdin === "-" || !process.stdin.isTTY) {
    // Read from stdin
    const stdin = await Bun.stdin.text();
    body = stdin;
    contentType = contentType || "text/plain";
  } else {
    console.error(`File not found: ${fileOrStdin}`);
    process.exit(1);
  }

  // Build URL with query params
  const url = new URL("/api/upload", config.server);
  if (filename) {
    url.searchParams.set("filename", filename);
  }
  if (options.raw) {
    url.searchParams.set("render", "raw");
  }

  try {
    const response = await fetch(url.toString(), {
      method: "POST",
      headers: {
        "Authorization": `Bearer ${config.token}`,
        "Content-Type": contentType || "application/octet-stream",
      },
      body,
    });

    if (!response.ok) {
      const text = await response.text();
      console.error(`Upload failed: ${response.status} ${text}`);
      process.exit(1);
    }

    const result = await response.text();
    // Output just the URL (for piping)
    process.stdout.write(result);
  } catch (e: any) {
    console.error(`Upload failed: ${e.message}`);
    process.exit(1);
  }
}

function detectContentType(filename: string): string {
  const ext = extname(filename).toLowerCase();

  const mimeTypes: Record<string, string> = {
    // Text/Code
    ".txt": "text/plain",
    ".md": "text/markdown",
    ".markdown": "text/markdown",
    ".json": "application/json",
    ".yaml": "text/yaml",
    ".yml": "text/yaml",
    ".toml": "text/toml",
    ".xml": "application/xml",
    ".html": "text/html",
    ".htm": "text/html",
    ".css": "text/css",
    ".js": "text/javascript",
    ".ts": "text/typescript",
    ".tsx": "text/tsx",
    ".jsx": "text/jsx",
    ".py": "text/x-python",
    ".go": "text/x-go",
    ".rs": "text/x-rust",
    ".rb": "text/x-ruby",
    ".sh": "text/x-shellscript",
    ".bash": "text/x-shellscript",
    ".sql": "text/x-sql",
    ".java": "text/x-java",
    ".c": "text/x-c",
    ".h": "text/x-c",
    ".cpp": "text/x-cpp",
    ".hpp": "text/x-cpp",
    ".cs": "text/x-csharp",
    ".php": "text/x-php",
    ".swift": "text/x-swift",

    // Images
    ".png": "image/png",
    ".jpg": "image/jpeg",
    ".jpeg": "image/jpeg",
    ".gif": "image/gif",
    ".svg": "image/svg+xml",
    ".webp": "image/webp",
    ".ico": "image/x-icon",

    // Documents
    ".pdf": "application/pdf",
    ".doc": "application/msword",
    ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",

    // Archives
    ".zip": "application/zip",
    ".tar": "application/x-tar",
    ".gz": "application/gzip",

    // Other
    ".wasm": "application/wasm",
  };

  return mimeTypes[ext] || "application/octet-stream";
}
