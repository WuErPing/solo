import { isAbsolutePath } from "./path";

interface BuildAbsoluteExplorerPathInput {
  workspaceRoot: string;
  entryPath: string;
}

export function buildAbsoluteExplorerPath({
  workspaceRoot,
  entryPath,
}: BuildAbsoluteExplorerPathInput): string {
  const normalizedWorkspaceRoot = workspaceRoot.trim().replace(/[\\/]+$/, "");
  const normalizedEntryPath = entryPath.trim();

  if (!normalizedWorkspaceRoot) {
    return normalizedEntryPath;
  }

  if (!normalizedEntryPath || normalizedEntryPath === ".") {
    return normalizedWorkspaceRoot;
  }

  if (isAbsolutePath(normalizedEntryPath)) {
    return normalizedEntryPath;
  }

  const separator = normalizedWorkspaceRoot.includes("\\") ? "\\" : "/";
  const segments = normalizedEntryPath.split(/[\\/]+/).filter(Boolean);
  if (segments.length === 0) {
    return normalizedWorkspaceRoot;
  }

  return `${normalizedWorkspaceRoot}${separator}${segments.join(separator)}`;
}

interface RestorePersistedDirectoryPathsInput {
  persistedPaths: readonly string[] | undefined;
  requestDirectoryListing: (
    path: string,
    options: { recordHistory: false; setCurrentPath: false },
  ) => Promise<boolean>;
}

export async function restorePersistedDirectoryPaths({
  persistedPaths,
  requestDirectoryListing,
}: RestorePersistedDirectoryPathsInput): Promise<string[] | null> {
  const nestedPaths = persistedPaths?.filter((path) => path.trim() && path !== ".") ?? [];
  if (nestedPaths.length === 0) {
    return null;
  }

  const results = await Promise.all(
    nestedPaths.map(async (path) => ({
      path,
      succeeded: await requestDirectoryListing(path, {
        recordHistory: false,
        setCurrentPath: false,
      }),
    })),
  );
  const restoredPaths = results.filter((result) => result.succeeded).map((result) => result.path);

  if (restoredPaths.length !== nestedPaths.length) {
    await requestDirectoryListing(".", {
      recordHistory: false,
      setCurrentPath: false,
    });
  }

  return [".", ...restoredPaths];
}
