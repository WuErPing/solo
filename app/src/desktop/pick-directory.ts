import { getDesktopHost } from "@/desktop/host";

export async function pickDirectory(): Promise<string | null> {
  const open = getDesktopHost()?.dialog?.open;
  if (typeof open !== "function") {
    // Fallback for web browsers: prompt user to enter a path
    const path = window.prompt("Enter the absolute path to your project directory:");
    if (path === null || path.trim() === "") {
      return null;
    }
    return path.trim();
  }

  const selection = await open({
    directory: true,
    multiple: false,
  });

  if (selection === null) {
    return null;
  }

  if (typeof selection === "string") {
    return selection;
  }

  throw new Error("Unexpected directory picker response.");
}
