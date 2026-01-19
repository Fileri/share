import { loadConfig } from "../config";

export async function deleteItem(id: string): Promise<void> {
  const config = await loadConfig();

  if (!config.token) {
    console.error("No token configured. Set SHARE_TOKEN or configure ~/.config/share/config.yaml");
    process.exit(1);
  }

  try {
    const response = await fetch(`${config.server}/api/delete/${id}`, {
      method: "DELETE",
      headers: {
        "Authorization": `Bearer ${config.token}`,
      },
    });

    if (response.status === 404) {
      console.error(`Not found: ${id}`);
      process.exit(1);
    }

    if (response.status === 403) {
      console.error(`Forbidden: you don't own this item`);
      process.exit(1);
    }

    if (!response.ok) {
      const text = await response.text();
      console.error(`Delete failed: ${response.status} ${text}`);
      process.exit(1);
    }

    console.log(`Deleted: ${id}`);
  } catch (e: any) {
    console.error(`Delete failed: ${e.message}`);
    process.exit(1);
  }
}
