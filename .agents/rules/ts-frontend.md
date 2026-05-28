# TypeScript & React Native Frontend Rules

Rules for `app/`, `app-bridge/`, and `packages/highlight/`.

## Type Safety

- TypeScript strict mode is enabled. Never use `any` except at third-party library boundaries where no type definitions exist, and always add a `// TODO: type this` comment.
- Prefer `unknown` over `any` for untyped inputs; narrow with type guards before use.
- Use discriminated unions for state variants over optional fields:
  ```ts
  // Good
  type Status = { kind: "loading" } | { kind: "loaded"; data: T } | { kind: "error"; message: string };
  // Bad
  type Status = { loading: boolean; data?: T; error?: string };
  ```
- Avoid type assertions (`as`). If needed, prefer a user-defined type guard function.
- Use `readonly` for arrays and objects that should not be mutated.
- Use `satisfies` operator to validate object shapes without widening types.

## Components

- Functional components only. No class components.
- Props interfaces: define inline for components used once, extract to a named type if reused.
- Avoid default exports except for Expo Router page files (`app/src/app/**/*.tsx`).
- Keep components under 200 lines. Extract sub-components or hooks when a component grows.
- Use `React.memo` only when profiling shows re-render is a bottleneck; never apply preemptively.
- Colocate styles, types, and tests with the component they belong to.

## State Management

- **Zustand stores** (`app/src/stores/`): Use for client-local state shared across screens. Keep stores small and focused. One store per domain (e.g., `useSessionStore`, `useSettingsStore`).
- **React Query** (`@tanstack/react-query`): Use for all server-derived state (API data, daemon responses, session timelines). Never duplicate server data in Zustand.
- **React Context** (`app/src/contexts/`): Use sparingly, only for values that rarely change and need deep tree access (theme, auth). Context triggers re-renders on every value change.
- **Local state** (`useState`/`useReducer`): Use for component-scoped UI state (form inputs, toggles, modals).
- Never store derived data in state. Compute it during render or with `useMemo`.

## Hooks

- Custom hooks go in `app/src/hooks/`. Prefix with `use`.
- Hooks must not have side effects outside `useEffect`. Data fetching belongs in React Query hooks.
- Always include dependencies in `useEffect` and `useCallback` dependency arrays. Disable the lint rule only with a comment explaining why.
- Return stable references from custom hooks when consumers depend on identity (use `useCallback`/`useMemo` internally).

## Styling

- Use Unistyles (`react-native-unistyles`) for all styling. No inline `StyleSheet.create` in new code.
- Theme-aware styles use the Unistyles theme object. Never hardcode colors; use theme tokens.
- Responsive layouts: use `flex` and `gap`. Avoid fixed pixel widths. Use `useWindowDimensions` for breakpoint logic.
- Platform-specific styles: use `Platform.select` or Unistyles variants, not separate style files.

## Async Patterns

- Always handle promise rejections. Use `try`/`catch` in async functions; attach `.catch()` to fire-and-forget promises.
- For daemon communication, use the `DaemonClient` from `app-bridge`. Never open raw WebSocket connections in components.
- Cancellation: pass `AbortSignal` to fetch calls when the component may unmount before the request completes.
- Avoid nested callbacks. Use `async`/`await` throughout.

## Imports

- Use path aliases: `@/` for `app/src/`, `@server/` for `app-bridge/src/`.
- Import order: external packages first, then internal `@/` imports, then relative imports. Separate groups with a blank line.
- Tree-shakeable imports: prefer named imports over default imports for libraries that support it (e.g., `import { Button } from 'react-native'` patterns).

## Expo & React Native Specifics

- File-system routing: page files in `app/src/app/` follow Expo Router conventions. Layout files (`_layout.tsx`) define navigation structure.
- Never block the JS thread with heavy computation. Offload to workers or native modules.
- Use `expo-linking` for deep links. Use `expo-constants` for build-time config.
- Platform checks: `Platform.OS === 'web'` for web-only code paths; guard native module access.
- Terminal: `@xterm/xterm` v6 for terminal rendering. Terminal addons go in `app/src/terminal/`.

## App-Bridge Specifics

- `app-bridge` is a pure TypeScript library with no React dependencies. Keep it framework-agnostic.
- E2EE crypto code lives in `app-bridge/src/relay/`. Never move crypto operations to the app layer.
- Export types alongside functions. Consumers should be able to import types without runtime side effects.
- The bridge communicates with the daemon via WebSocket. All messages must go through the typed protocol definitions.
