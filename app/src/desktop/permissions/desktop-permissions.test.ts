import { afterEach, describe, expect, it, vi } from "vitest";

type MockPlatform = "web" | "ios" | "android";

interface GlobalSnapshot {
  Notification: unknown;
  navigatorDescriptor?: PropertyDescriptor;
  windowDescriptor?: PropertyDescriptor;
  soloDesktop: unknown;
}

const originalGlobals: GlobalSnapshot = {
  Notification: (globalThis as { Notification?: unknown }).Notification,
  navigatorDescriptor: Object.getOwnPropertyDescriptor(globalThis, "navigator"),
  windowDescriptor: Object.getOwnPropertyDescriptor(globalThis, "window"),
  soloDesktop:
    typeof globalThis.window === "undefined"
      ? undefined
      : (globalThis.window as { soloDesktop?: unknown }).soloDesktop,
};

function ensureWindow(): { soloDesktop?: unknown } {
  const existingWindow = (globalThis as { window?: { soloDesktop?: unknown } }).window;
  if (existingWindow) {
    return existingWindow;
  }

  const nextWindow: { soloDesktop?: unknown } = {};
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    writable: true,
    value: nextWindow,
  });
  return nextWindow;
}

function setNavigator(value: unknown): void {
  Object.defineProperty(globalThis, "navigator", {
    configurable: true,
    writable: true,
    value,
  });
}

function restoreGlobals(): void {
  (globalThis as { Notification?: unknown }).Notification = originalGlobals.Notification;

  if (originalGlobals.navigatorDescriptor) {
    Object.defineProperty(globalThis, "navigator", originalGlobals.navigatorDescriptor);
  } else {
    delete (globalThis as { navigator?: unknown }).navigator;
  }

  if (originalGlobals.windowDescriptor) {
    Object.defineProperty(globalThis, "window", originalGlobals.windowDescriptor);
  } else {
    delete (globalThis as { window?: unknown }).window;
  }

  if (typeof globalThis.window !== "undefined") {
    (globalThis.window as { soloDesktop?: unknown }).soloDesktop = originalGlobals.soloDesktop;
  }
}

async function loadModuleForPlatform(platform: MockPlatform) {
  vi.resetModules();
  vi.doMock("react-native", () => ({ Platform: { OS: platform } }));
  return import("./desktop-permissions");
}

describe("desktop-permissions", () => {
  afterEach(() => {
    vi.doUnmock("react-native");
    vi.restoreAllMocks();
    vi.resetModules();
    restoreGlobals();
  });

  it("shows section only in desktop web runtime", async () => {
    const { shouldShowDesktopPermissionSection } = await loadModuleForPlatform("web");

    expect(shouldShowDesktopPermissionSection()).toBe(false);

    ensureWindow().soloDesktop = {};
    expect(shouldShowDesktopPermissionSection()).toBe(true);
  });

  it("reads notification status", async () => {
    const MockNotification = { permission: "default" };
    (globalThis as { Notification?: unknown }).Notification = MockNotification;
    setNavigator({
      permissions: {
        query: vi.fn(async () => ({ state: "granted" })),
      },
    });

    const { getDesktopPermissionSnapshot } = await loadModuleForPlatform("web");
    const snapshot = await getDesktopPermissionSnapshot();

    expect(snapshot.notifications.state).toBe("prompt");
    expect(snapshot.checkedAt).toBeTypeOf("number");
  });

  it("requests notification permission via the browser Notification API", async () => {
    const MockNotification = {
      permission: "default",
      requestPermission: vi.fn(async () => "granted"),
    };
    (globalThis as { Notification?: unknown }).Notification = MockNotification;

    const { requestDesktopPermission } = await loadModuleForPlatform("web");
    const result = await requestDesktopPermission({ kind: "notifications" });

    expect(result.state).toBe("granted");
    expect(MockNotification.requestPermission).toHaveBeenCalledTimes(1);
  });

  it("reads browser Notification permission when available", async () => {
    const MockNotification = { permission: "denied" };
    (globalThis as { Notification?: unknown }).Notification = MockNotification;
    setNavigator({});

    const { getDesktopPermissionSnapshot } = await loadModuleForPlatform("web");
    const snapshot = await getDesktopPermissionSnapshot();

    expect(snapshot.notifications.state).toBe("denied");
  });
});
