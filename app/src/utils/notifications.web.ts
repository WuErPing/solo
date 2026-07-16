/**
 * Web stub for expo-notifications (see notifications.ts for why).
 * Push notifications are unsupported on web; all APIs are no-ops.
 */

export const AndroidImportance = {
  DEFAULT: 3,
} as const;

export function setNotificationHandler(): void {}

export function addNotificationResponseReceivedListener(): { remove: () => void } {
  return { remove: () => {} };
}

export async function getLastNotificationResponseAsync(): Promise<null> {
  return null;
}

export async function getPermissionsAsync(): Promise<{ status: string; canAskAgain: boolean }> {
  return { status: "denied", canAskAgain: false };
}

export async function requestPermissionsAsync(): Promise<{ status: string }> {
  return { status: "denied" };
}

export async function setNotificationChannelAsync(): Promise<null> {
  return null;
}

export async function getExpoPushTokenAsync(): Promise<{ data: string }> {
  throw new Error("Expo push tokens are not supported on web");
}
