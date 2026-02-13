import { platform } from "os";

export async function copyToClipboard(text: string): Promise<boolean> {
  const os = platform();

  let cmd: string[];

  if (os === "darwin") {
    cmd = ["pbcopy"];
  } else if (os === "linux") {
    // Check if we're in WSL
    if (process.env.WSL_DISTRO_NAME) {
      cmd = ["clip.exe"];
    } else {
      // Try xclip first, fall back to xsel
      cmd = ["xclip", "-selection", "clipboard"];
    }
  } else if (os === "win32") {
    cmd = ["clip"];
  } else {
    return false;
  }

  try {
    const proc = Bun.spawn(cmd, {
      stdin: "pipe",
      stdout: "ignore",
      stderr: "ignore",
    });

    proc.stdin.write(text);
    proc.stdin.end();

    const exitCode = await proc.exited;
    return exitCode === 0;
  } catch {
    return false;
  }
}
