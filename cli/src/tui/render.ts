import type { ListItem } from "../types";
import { BANNER } from "./banner";

// ANSI escape codes
export const ANSI = {
  // Cursor
  hideCursor: "\x1b[?25l",
  showCursor: "\x1b[?25h",
  moveTo: (row: number, col: number) => `\x1b[${row};${col}H`,
  moveToTop: "\x1b[H",

  // Screen
  clearScreen: "\x1b[2J",
  clearLine: "\x1b[2K",

  // Colors
  reset: "\x1b[0m",
  bold: "\x1b[1m",
  dim: "\x1b[2m",
  inverse: "\x1b[7m",

  // Foreground
  fg: {
    black: "\x1b[30m",
    red: "\x1b[31m",
    green: "\x1b[32m",
    yellow: "\x1b[33m",
    blue: "\x1b[34m",
    magenta: "\x1b[35m",
    cyan: "\x1b[36m",
    white: "\x1b[37m",
    gray: "\x1b[90m",
  },
};

export interface TUIState {
  items: ListItem[];
  selectedIndex: number;
  loading: boolean;
  error: string | null;
  confirmDelete: boolean;
  message: string | null;
}

export function render(state: TUIState): void {
  const { columns = 80, rows = 24 } = process.stdout;
  const output: string[] = [];

  // Clear screen and move to top
  output.push(ANSI.clearScreen + ANSI.moveToTop);

  // Banner
  output.push(ANSI.fg.cyan + ANSI.bold + BANNER + ANSI.reset);

  // Help line
  output.push(
    ANSI.dim +
      "[↑/↓] Navigate  [Enter] Copy URL  [d] Delete  [r] Refresh  [q] Quit" +
      ANSI.reset
  );
  output.push("");

  // Status message
  if (state.message) {
    output.push(ANSI.fg.green + state.message + ANSI.reset);
    output.push("");
  }

  // Error message
  if (state.error) {
    output.push(ANSI.fg.red + "Error: " + state.error + ANSI.reset);
    output.push("");
  }

  // Loading state
  if (state.loading) {
    output.push(ANSI.dim + "Loading..." + ANSI.reset);
    process.stdout.write(output.join("\n"));
    return;
  }

  // Delete confirmation
  if (state.confirmDelete && state.items[state.selectedIndex]) {
    const item = state.items[state.selectedIndex];
    output.push(
      ANSI.fg.yellow +
        `Delete "${item.filename || item.id}"? [y/n]` +
        ANSI.reset
    );
    output.push("");
  }

  // Empty state
  if (state.items.length === 0) {
    output.push(ANSI.dim + "No uploads found." + ANSI.reset);
    process.stdout.write(output.join("\n"));
    return;
  }

  // Header
  const filenameWidth = Math.max(20, columns - 40);
  output.push(
    ANSI.bold +
      padRight("FILENAME", filenameWidth) +
      padRight("SIZE", 12) +
      "CREATED" +
      ANSI.reset
  );
  output.push(ANSI.dim + "─".repeat(Math.min(columns - 2, 80)) + ANSI.reset);

  // Items
  for (let i = 0; i < state.items.length; i++) {
    const item = state.items[i];
    const isSelected = i === state.selectedIndex;

    const filename = truncate(item.filename || item.id, filenameWidth - 2);
    const size = formatSize(item.size);
    const created = formatDate(item.created);

    const prefix = isSelected ? "> " : "  ";
    const line =
      prefix +
      padRight(filename, filenameWidth - 2) +
      padRight(size, 12) +
      created;

    if (isSelected) {
      output.push(ANSI.inverse + line + ANSI.reset);
    } else {
      output.push(line);
    }
  }

  // Footer
  output.push("");
  output.push(
    ANSI.dim +
      `${state.items.length} item${state.items.length === 1 ? "" : "s"}` +
      ANSI.reset
  );

  process.stdout.write(output.join("\n"));
}

function padRight(str: string, len: number): string {
  if (str.length >= len) return str;
  return str + " ".repeat(len - str.length);
}

function truncate(str: string, maxLen: number): string {
  if (str.length <= maxLen) return str;
  return str.slice(0, maxLen - 1) + "…";
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

  if (diff < 24 * 60 * 60 * 1000) {
    const hours = Math.floor(diff / (60 * 60 * 1000));
    if (hours === 0) {
      const minutes = Math.floor(diff / (60 * 1000));
      return `${minutes}m ago`;
    }
    return `${hours}h ago`;
  }

  return date.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: date.getFullYear() !== now.getFullYear() ? "numeric" : undefined,
  });
}
