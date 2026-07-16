/**
 * Platform indirection for expo-notifications.
 *
 * On web, importing expo-notifications eagerly registers a device push-token
 * listener that only warns ("not yet fully supported on web") and can never
 * work. Metro resolves this file to notifications.web.ts on web, so the
 * native module is never loaded there. Native platforms get the real module.
 */
export * from "expo-notifications";
