import { loadConfig } from "../config";
import type { ListItem } from "../types";

export async function list(): Promise<void> {
  const config = await loadConfig();

  if (!config.token) {
    console.error("No token configured. Set SHARE_TOKEN or configure ~/.config/share/config.yaml");
    process.exit(1);
  }

  try {
    const response = await fetch(`${config.server}/api/list`, {
      headers: {
        "Authorization": `Bearer ${config.token}`,
      },
    });

    if (!response.ok) {
      const text = await response.text();
      console.error(`List failed: ${response.status} ${text}`);
      process.exit(1);
    }

    const items: ListItem[] = await response.json();

    if (items.length === 0) {
      console.log("No uploads found.");
      return;
    }

    // Print table
    console.log("");
    console.log("ID                        SIZE       CREATED              FILENAME");
    console.log("─".repeat(80));

    for (const item of items) {
      const id = item.id.padEnd(24);
      const size = formatSize(item.size).padStart(10);
      const created = formatDate(item.created).padEnd(20);
      const filename = item.filename || "-";
      console.log(`${id}  ${size}  ${created}  ${filename}`);
    }

    console.log("");
    console.log(`Total: ${items.length} item${items.length === 1 ? "" : "s"}`);
  } catch (e: any) {
    console.error(`List failed: ${e.message}`);
    process.exit(1);
  }
}

function formatSize(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const size = bytes / Math.pow(1024, i);
  return `${size.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

function formatDate(isoDate: string): string {
  const date = new Date(isoDate);
  const now = new Date();
  const diff = now.getTime() - date.getTime();

  // If less than 24 hours, show relative time
  if (diff < 24 * 60 * 60 * 1000) {
    const hours = Math.floor(diff / (60 * 60 * 1000));
    if (hours === 0) {
      const minutes = Math.floor(diff / (60 * 1000));
      return `${minutes}m ago`;
    }
    return `${hours}h ago`;
  }

  // Otherwise show date
  return date.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: date.getFullYear() !== now.getFullYear() ? "numeric" : undefined,
  });
}
