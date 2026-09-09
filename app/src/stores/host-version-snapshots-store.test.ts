import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const asyncStorageMock = vi.hoisted(() => ({
  getItem: vi.fn<(_: string) => Promise<string | null>>(async () => null),
  setItem: vi.fn<(_: string, __: string) => Promise<void>>(async () => {}),
}));

vi.mock("@react-native-async-storage/async-storage", () => ({
  default: asyncStorageMock,
}));

const STORAGE_KEY = "@solo:host-version-snapshots";

const validSnapshot = {
  versions: [
    { version: "solo-v0.12.0", mtimeMs: 200 },
    { version: "solo-v0.11.0", mtimeMs: 100 },
  ],
  currentVersion: "solo-v0.12.0",
  runningVersion: "v0.12.0",
  supervisor: {
    state: "running",
    pid: 42,
    spawnedBinary: "solo-v0.12.0",
    pointerVersion: "solo-v0.12.0",
    consecutiveCrashes: 0,
    backoffMs: null,
    lastExitCode: null,
    updatedAtMs: 1700000000000,
    lastEvent: "spawn pid=42",
  },
  fetchedAt: "2026-08-27T00:00:00.000Z",
};

async function importStoreFresh(stored: string | null) {
  vi.resetModules();
  asyncStorageMock.getItem.mockResolvedValue(stored);
  return await import("@/stores/host-version-snapshots-store");
}

describe("host-version-snapshots-store", () => {
  beforeEach(() => {
    asyncStorageMock.getItem.mockResolvedValue(null);
    asyncStorageMock.setItem.mockClear();
  });

  afterEach(() => {
    vi.resetModules();
  });

  it("saves and returns a snapshot per server", async () => {
    const store = await importStoreFresh(null);
    store.saveHostVersionSnapshot("srv-1", validSnapshot);
    const saved = store.getHostVersionSnapshot("srv-1");
    expect(saved).toEqual({ ...validSnapshot, fetchedAt: saved?.fetchedAt });
    expect(saved?.fetchedAt).not.toBe(validSnapshot.fetchedAt);
    expect(store.getHostVersionSnapshot("srv-2")).toBeNull();

    const persisted = JSON.parse(
      asyncStorageMock.setItem.mock.calls.find(([key]) => key === STORAGE_KEY)?.[1] ?? "null",
    );
    expect(persisted["srv-1"]).toEqual(saved);
  });

  it("stamps fetchedAt on save", async () => {
    const store = await importStoreFresh(null);
    const before = new Date("2026-08-27T00:00:00.000Z").getTime();
    store.saveHostVersionSnapshot("srv-1", validSnapshot);
    const fetchedAt = store.getHostVersionSnapshot("srv-1")?.fetchedAt ?? "";
    expect(new Date(fetchedAt).getTime()).toBeGreaterThanOrEqual(before);
  });

  it("replaces the snapshot for the same server and keeps others", async () => {
    const store = await importStoreFresh(null);
    store.saveHostVersionSnapshot("srv-1", validSnapshot);
    store.saveHostVersionSnapshot("srv-2", validSnapshot);
    store.saveHostVersionSnapshot("srv-1", { ...validSnapshot, runningVersion: "v0.13.0" });
    expect(store.getHostVersionSnapshot("srv-1")?.runningVersion).toBe("v0.13.0");
    expect(store.getHostVersionSnapshot("srv-2")?.runningVersion).toBe("v0.12.0");
  });

  it("clears a snapshot and persists the removal", async () => {
    const store = await importStoreFresh(null);
    store.saveHostVersionSnapshot("srv-1", validSnapshot);
    asyncStorageMock.setItem.mockClear();
    store.clearHostVersionSnapshot("srv-1");
    expect(store.getHostVersionSnapshot("srv-1")).toBeNull();
    expect(asyncStorageMock.setItem).toHaveBeenCalledWith(STORAGE_KEY, "{}");
    // Unknown ids are a no-op.
    store.clearHostVersionSnapshot("srv-unknown");
    expect(asyncStorageMock.setItem).toHaveBeenCalledTimes(1);
  });

  it("hydrates a persisted record and drops invalid entries", async () => {
    const stored = JSON.stringify({
      "srv-1": validSnapshot,
      "srv-bad": { versions: "nope", fetchedAt: "" },
      "srv-partial": { versions: [{ version: "solo-x", mtimeMs: 5 }], fetchedAt: "2026-01-01T00:00:00Z" },
    });
    const store = await importStoreFresh(stored);
    expect(store.getHostVersionSnapshot("srv-1")).toEqual(validSnapshot);
    expect(store.getHostVersionSnapshot("srv-bad")).toBeNull();
    expect(store.getHostVersionSnapshot("srv-partial")).toEqual({
      versions: [{ version: "solo-x", mtimeMs: 5 }],
      currentVersion: null,
      runningVersion: null,
      supervisor: null,
      fetchedAt: "2026-01-01T00:00:00Z",
    });
  });

  it("hydrates to an empty record for corrupt storage", async () => {
    const store = await importStoreFresh("{not json");
    expect(store.getHostVersionSnapshot("srv-1")).toBeNull();
  });

  it("caps persisted versions per snapshot", async () => {
    const store = await importStoreFresh(null);
    const manyVersions = Array.from({ length: 40 }, (_, index) => ({
      version: `solo-v${index}`,
      mtimeMs: index,
    }));
    store.saveHostVersionSnapshot("srv-1", { ...validSnapshot, versions: manyVersions });
    expect(store.getHostVersionSnapshot("srv-1")?.versions).toHaveLength(16);
  });
});
