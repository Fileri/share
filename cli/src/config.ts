import { existsSync, readFileSync } from "fs";
import { homedir } from "os";
import { join } from "path";
import { parse } from "yaml";

export interface Config {
  server: string;
  token: string;
}

const CONFIG_PATHS = [
  join(homedir(), ".config", "share", "config.yaml"),
  join(homedir(), ".config", "share", "config.yml"),
  join(homedir(), ".share.yaml"),
];

export async function loadConfig(): Promise<Config> {
  // Check environment variables first
  const envServer = process.env.SHARE_SERVER;
  const envToken = process.env.SHARE_TOKEN;

  if (envServer && envToken) {
    return { server: envServer, token: envToken };
  }

  // Try config files
  for (const path of CONFIG_PATHS) {
    if (existsSync(path)) {
      try {
        const content = readFileSync(path, "utf-8");
        const config = parse(content) as Partial<Config>;

        // Resolve 1Password references
        let token = config.token || envToken || "";
        if (token.startsWith("op://")) {
          token = await resolve1Password(token);
        }

        return {
          server: config.server || envServer || "http://localhost:8080",
          token: token,
        };
      } catch (e) {
        // Continue to next config file
      }
    }
  }

  // Defaults
  return {
    server: envServer || "http://localhost:8080",
    token: envToken || "",
  };
}

async function resolve1Password(ref: string): Promise<string> {
  try {
    const proc = Bun.spawn(["op", "read", ref], {
      stdout: "pipe",
      stderr: "pipe",
    });
    const output = await new Response(proc.stdout).text();
    return output.trim();
  } catch (e) {
    console.error(`Failed to resolve 1Password reference: ${ref}`);
    console.error("Make sure 1Password CLI is installed and you're signed in");
    process.exit(1);
  }
}
