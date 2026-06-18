import { getDesktopHost } from "@/desktop/host";
import { isWeb, isNative } from "@/constants/platform";

export type DesktopPermissionKind = "notifications";

export type DesktopPermissionState =
  | "granted"
  | "denied"
  | "prompt"
  | "not-granted"
  | "unavailable"
  | "unknown";

export interface DesktopPermissionStatus {
  state: DesktopPermissionState;
  detail: string;
}

export interface DesktopPermissionSnapshot {
  checkedAt: number;
  notifications: DesktopPermissionStatus;
}

interface NotificationConstructorLike {
  permission?: string;
  requestPermission?: () => Promise<string>;
}

export function shouldShowDesktopPermissionSection(): boolean {
  return isWeb && getDesktopHost() !== null;
}

function status(input: DesktopPermissionStatus): DesktopPermissionStatus {
  return input;
}

function getErrorMessage(error: unknown): string {
  if (error instanceof Error && typeof error.message === "string") {
    return error.message;
  }
  return String(error);
}

function getWebNotificationConstructor(): NotificationConstructorLike | null {
  if (isNative) {
    return null;
  }
  const NotificationConstructor = (globalThis as { Notification?: unknown }).Notification;
  if (
    NotificationConstructor == null ||
    (typeof NotificationConstructor !== "function" && typeof NotificationConstructor !== "object")
  ) {
    return null;
  }
  return NotificationConstructor as NotificationConstructorLike;
}

function mapNotificationPermissionString(permission: string): DesktopPermissionStatus {
  if (permission === "granted") {
    return status({
      state: "granted",
      detail: "Notifications are allowed by the OS.",
    });
  }
  if (permission === "denied") {
    return status({
      state: "denied",
      detail: "Notifications are denied in system settings.",
    });
  }
  if (permission === "default") {
    return status({
      state: "prompt",
      detail: "Notifications have not been granted yet.",
    });
  }
  return status({
    state: "unknown",
    detail: `Unexpected notification permission state: ${permission}`,
  });
}

async function getNotificationPermissionStatus(): Promise<DesktopPermissionStatus> {
  if (isNative) {
    return status({
      state: "unavailable",
      detail: "Desktop notification status is only available on web runtime.",
    });
  }

  const desktopHost = getDesktopHost();
  if (desktopHost && typeof desktopHost.notification?.isSupported === "function") {
    try {
      const supported = await desktopHost.notification.isSupported();
      return status({
        state: supported ? "granted" : "unavailable",
        detail: supported
          ? "Desktop notifications are supported."
          : "Desktop notifications are not supported on this platform.",
      });
    } catch {
      // Fall through to web API check
    }
  }

  const NotificationConstructor = getWebNotificationConstructor();
  if (NotificationConstructor && typeof NotificationConstructor.permission === "string") {
    return mapNotificationPermissionString(NotificationConstructor.permission);
  }

  return status({
    state: "unavailable",
    detail: "Web Notification API is unavailable in this environment.",
  });
}

async function requestNotificationPermissionStatus(): Promise<DesktopPermissionStatus> {
  if (isNative) {
    return status({
      state: "unavailable",
      detail: "Desktop notification requests are only available on web runtime.",
    });
  }

  const NotificationConstructor = getWebNotificationConstructor();
  if (NotificationConstructor && typeof NotificationConstructor.requestPermission === "function") {
    try {
      const permission = await NotificationConstructor.requestPermission();
      return mapNotificationPermissionString(permission);
    } catch (error) {
      return status({
        state: "unknown",
        detail: `Failed to request notification permission: ${getErrorMessage(error)}`,
      });
    }
  }

  return status({
    state: "unavailable",
    detail: "Web Notification API requestPermission() is unavailable.",
  });
}

export async function requestDesktopPermission(input: {
  kind: DesktopPermissionKind;
}): Promise<DesktopPermissionStatus> {
  return await requestNotificationPermissionStatus();
}

export async function getDesktopPermissionSnapshot(): Promise<DesktopPermissionSnapshot> {
  const notifications = await getNotificationPermissionStatus();

  return {
    checkedAt: Date.now(),
    notifications,
  };
}
