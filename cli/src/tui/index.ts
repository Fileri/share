import type { ListItem } from "../types";
import { loadConfig } from "../config";
import { ANSI, render, type TUIState } from "./render";
import { enableRawMode, disableRawMode, onKeypress, type Key } from "./input";
import { copyToClipboard } from "./clipboard";

export async function runTUI(): Promise<void> {
  const config = await loadConfig();

  if (!config.token) {
    console.error(
      "No token configured. Set SHARE_TOKEN or configure ~/.config/share/config.yaml"
    );
    process.exit(1);
  }

  const state: TUIState = {
    items: [],
    selectedIndex: 0,
    loading: true,
    error: null,
    confirmDelete: false,
    message: null,
  };

  // Setup cleanup
  const cleanup = () => {
    disableRawMode();
    process.stdout.write(ANSI.showCursor);
    process.stdout.write(ANSI.clearScreen + ANSI.moveToTop);
  };

  process.on("exit", cleanup);
  process.on("SIGINT", () => {
    cleanup();
    process.exit(0);
  });
  process.on("SIGTERM", () => {
    cleanup();
    process.exit(0);
  });

  // Hide cursor and enable raw mode
  process.stdout.write(ANSI.hideCursor);
  enableRawMode();

  // Fetch initial data
  await fetchItems(config.server, config.token, state);
  render(state);

  // Handle keyboard input
  const removeListener = onKeypress(async (key: Key) => {
    // Clear transient messages
    state.message = null;

    if (key === "ctrl-c" || key === "q") {
      removeListener();
      cleanup();
      process.exit(0);
    }

    // Delete confirmation mode
    if (state.confirmDelete) {
      if (key === "y") {
        const item = state.items[state.selectedIndex];
        if (item) {
          await deleteItem(config.server, config.token, item.id, state);
        }
        state.confirmDelete = false;
      } else if (key === "n" || key === "escape") {
        state.confirmDelete = false;
      }
      render(state);
      return;
    }

    // Normal mode
    switch (key) {
      case "up":
        if (state.selectedIndex > 0) {
          state.selectedIndex--;
        }
        break;

      case "down":
        if (state.selectedIndex < state.items.length - 1) {
          state.selectedIndex++;
        }
        break;

      case "enter":
        if (state.items[state.selectedIndex]) {
          const item = state.items[state.selectedIndex];
          const success = await copyToClipboard(item.url);
          if (success) {
            state.message = `Copied: ${item.url}`;
          } else {
            // Clipboard unavailable (headless/SSH) - show URL for manual copy
            state.message = `URL: ${item.url}`;
          }
        }
        break;

      case "d":
        if (state.items[state.selectedIndex]) {
          state.confirmDelete = true;
        }
        break;

      case "r":
        state.loading = true;
        render(state);
        await fetchItems(config.server, config.token, state);
        break;
    }

    render(state);
  });

  // Keep process alive
  await new Promise(() => {});
}

async function fetchItems(
  server: string,
  token: string,
  state: TUIState
): Promise<void> {
  state.loading = true;
  state.error = null;

  try {
    const response = await fetch(`${server}/api/list`, {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    if (!response.ok) {
      const text = await response.text();
      state.error = `${response.status} ${text}`;
      state.loading = false;
      return;
    }

    const items: ListItem[] = await response.json();
    state.items = items;
    state.loading = false;

    // Clamp selection to valid range
    if (state.selectedIndex >= items.length) {
      state.selectedIndex = Math.max(0, items.length - 1);
    }
  } catch (e: any) {
    state.error = e.message;
    state.loading = false;
  }
}

async function deleteItem(
  server: string,
  token: string,
  id: string,
  state: TUIState
): Promise<void> {
  try {
    const response = await fetch(`${server}/api/delete/${id}`, {
      method: "DELETE",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    if (!response.ok) {
      const text = await response.text();
      state.error = `Delete failed: ${response.status} ${text}`;
      return;
    }

    state.message = `Deleted: ${id}`;

    // Refresh list
    await fetchItems(server, token, state);
  } catch (e: any) {
    state.error = `Delete failed: ${e.message}`;
  }
}
