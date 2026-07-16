/**
 * useIsFocused that sees expo-router's navigator context.
 *
 * expo-router ships a *forked* react-navigation (build/react-navigation/*) and
 * its Stack navigators provide the fork's NavigationContext. Importing
 * useIsFocused from @react-navigation/native reads the real package's context,
 * which nothing provides — on web it throws
 * "Couldn't find a navigation object. Is your component inside NavigationContainer?"
 * Only the fork's own hook observes screen focus on every platform.
 */
export { useIsFocused } from "expo-router/build/react-navigation/core/useIsFocused";
