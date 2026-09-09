import AsyncStorage from "@react-native-async-storage/async-storage";
import { useSyncExternalStoreWithSelector } from "use-sync-external-store/shim/with-selector";
import type { SupervisorState } from "@server/generated/protocol-schemas";

const STORAGE_KEY = "@solo:host-version-snapshots";
const MAX_VERSIONS_PER_SNAPSHOT = 16;

export interface HostVersionSnapshotVersion {
  version: string;
  mtimeMs: number;
}

// Last-known-good view of the host's daemon versions, persisted so the app
// can still show what was available after the host becomes unreachable.
export interface HostVersionSnapshot {
  versions: HostVersionSnapshotVersion[];
  currentVersion: string | null;
  runningVersion: string | null;
  supervisor: SupervisorState | null;
  fetchedAt: string;
}

export interface HostVersionSnapshotInput {
  versions: readonly { version: string; mtimeMs: number }[];
  currentVersion?: string | null;
  runningVersion?: string | null;
  supervisor?: SupervisorState | null;
}

let snapshots: Record<string, HostVersionSnapshot> = {};
let revision = 0;
let hydrationPromise: Promise<void> | null = null;
const listeners = new Set<() => void>();

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function getSnapshotsRecord(): Record<string, HostVersionSnapshot> {
  return snapshots;
}

function notifyListeners() {
  for (const listener of listeners) {
    listener();
  }
}

function persist() {
  void AsyncStorage.setItem(STORAGE_KEY, JSON.stringify(snapshots)).catch(() => {});
}

function sanitizeVersionList(input: unknown): HostVersionSnapshotVersion[] {
  if (!Array.isArray(input)) {
    return [];
  }
  const versions: HostVersionSnapshotVersion[] = [];
  for (const entry of input) {
    if (!entry || typeof entry !== "object") {
      continue;
    }
    const record = entry as Record<string, unknown>;
    if (typeof record.version !== "string" || record.version.length === 0) {
      continue;
    }
    if (typeof record.mtimeMs !== "number" || !Number.isFinite(record.mtimeMs)) {
      continue;
    }
    versions.push({ version: record.version, mtimeMs: record.mtimeMs });
    if (versions.length >= MAX_VERSIONS_PER_SNAPSHOT) {
      break;
    }
  }
  return versions;
}

function sanitizeNullableString(value: unknown): string | null {
  return typeof value === "string" && value.length > 0 ? value : null;
}

function sanitizeSupervisorState(input: unknown): SupervisorState | null {
  if (!input || typeof input !== "object" || Array.isArray(input)) {
    return null;
  }
  const record = input as Record<string, unknown>;
  if (typeof record.state !== "string" || record.state.length === 0) {
    return null;
  }
  return {
    state: record.state,
    pid: typeof record.pid === "number" && Number.isFinite(record.pid) ? record.pid : 0,
    spawnedBinary: typeof record.spawnedBinary === "string" ? record.spawnedBinary : "",
    pointerVersion: sanitizeNullableString(record.pointerVersion),
    consecutiveCrashes:
      typeof record.consecutiveCrashes === "number" && Number.isFinite(record.consecutiveCrashes)
        ? record.consecutiveCrashes
        : 0,
    backoffMs:
      typeof record.backoffMs === "number" && Number.isFinite(record.backoffMs)
        ? record.backoffMs
        : null,
    lastExitCode:
      typeof record.lastExitCode === "number" && Number.isFinite(record.lastExitCode)
        ? record.lastExitCode
        : null,
    updatedAtMs:
      typeof record.updatedAtMs === "number" && Number.isFinite(record.updatedAtMs)
        ? record.updatedAtMs
        : 0,
    lastEvent: typeof record.lastEvent === "string" ? record.lastEvent : "",
  };
}

function sanitizeSnapshot(input: unknown): HostVersionSnapshot | null {
  if (!input || typeof input !== "object" || Array.isArray(input)) {
    return null;
  }
  const record = input as Record<string, unknown>;
  if (typeof record.fetchedAt !== "string" || record.fetchedAt.length === 0) {
    return null;
  }
  return {
    versions: sanitizeVersionList(record.versions),
    currentVersion: sanitizeNullableString(record.currentVersion),
    runningVersion: sanitizeNullableString(record.runningVersion),
    supervisor: sanitizeSupervisorState(record.supervisor),
    fetchedAt: record.fetchedAt,
  };
}

function sanitizeStoredRecord(stored: string | null): Record<string, HostVersionSnapshot> {
  if (!stored) {
    return {};
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(stored);
  } catch {
    return {};
  }
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    return {};
  }
  const record: Record<string, HostVersionSnapshot> = {};
  for (const [serverId, value] of Object.entries(parsed as Record<string, unknown>)) {
    if (!serverId) {
      continue;
    }
    const snapshot = sanitizeSnapshot(value);
    if (snapshot) {
      record[serverId] = snapshot;
    }
  }
  return record;
}

async function readSnapshotsFromStorage(hydrationRevision: number) {
  try {
    const stored = await AsyncStorage.getItem(STORAGE_KEY);
    if (revision === hydrationRevision) {
      snapshots = sanitizeStoredRecord(stored);
    }
  } catch {
    if (revision === hydrationRevision) {
      snapshots = {};
    }
  } finally {
    notifyListeners();
  }
}

export function hydrateHostVersionSnapshots(): Promise<void> {
  if (!hydrationPromise) {
    hydrationPromise = readSnapshotsFromStorage(revision);
  }
  return hydrationPromise;
}

export function saveHostVersionSnapshot(serverId: string, input: HostVersionSnapshotInput) {
  const trimmedServerId = serverId.trim();
  if (!trimmedServerId) {
    return;
  }
  const snapshot: HostVersionSnapshot = {
    versions: sanitizeVersionList(input.versions),
    currentVersion: sanitizeNullableString(input.currentVersion),
    runningVersion: sanitizeNullableString(input.runningVersion),
    supervisor: sanitizeSupervisorState(input.supervisor ?? null),
    fetchedAt: new Date().toISOString(),
  };
  revision += 1;
  snapshots = { ...snapshots, [trimmedServerId]: snapshot };
  persist();
  notifyListeners();
}

export function clearHostVersionSnapshot(serverId: string) {
  const trimmedServerId = serverId.trim();
  if (!trimmedServerId || !(trimmedServerId in snapshots)) {
    return;
  }
  revision += 1;
  const next = { ...snapshots };
  delete next[trimmedServerId];
  snapshots = next;
  persist();
  notifyListeners();
}

export function getHostVersionSnapshot(serverId: string): HostVersionSnapshot | null {
  return snapshots[serverId.trim()] ?? null;
}

export function useHostVersionSnapshot(serverId: string): HostVersionSnapshot | null {
  return useSyncExternalStoreWithSelector(
    subscribe,
    getSnapshotsRecord,
    getSnapshotsRecord,
    (record) => record[serverId.trim()] ?? null,
    Object.is,
  );
}

void hydrateHostVersionSnapshots();
