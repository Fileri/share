export type Key =
  | "up"
  | "down"
  | "enter"
  | "d"
  | "r"
  | "q"
  | "y"
  | "n"
  | "escape"
  | "ctrl-c"
  | "unknown";

export function enableRawMode(): void {
  if (process.stdin.isTTY) {
    process.stdin.setRawMode(true);
    process.stdin.resume();
  }
}

export function disableRawMode(): void {
  if (process.stdin.isTTY) {
    process.stdin.setRawMode(false);
    process.stdin.pause();
  }
}

export function parseKey(data: Buffer): Key {
  const str = data.toString();

  // Ctrl+C
  if (str === "\x03") return "ctrl-c";

  // Escape key or start of escape sequence
  if (str === "\x1b") return "escape";

  // Arrow keys (escape sequences)
  if (str === "\x1b[A" || str === "\x1bOA") return "up";
  if (str === "\x1b[B" || str === "\x1bOB") return "down";

  // Enter
  if (str === "\r" || str === "\n") return "enter";

  // Single chars
  const char = str.toLowerCase();
  if (char === "d") return "d";
  if (char === "r") return "r";
  if (char === "q") return "q";
  if (char === "y") return "y";
  if (char === "n") return "n";
  if (char === "j") return "down"; // vim-style
  if (char === "k") return "up"; // vim-style

  return "unknown";
}

export function onKeypress(callback: (key: Key) => void): () => void {
  const handler = (data: Buffer) => {
    const key = parseKey(data);
    callback(key);
  };

  process.stdin.on("data", handler);

  return () => {
    process.stdin.off("data", handler);
  };
}
