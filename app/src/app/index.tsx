import React from "react";
import { Redirect, usePathname, type Href } from "expo-router";
import { StartupSplashScreen } from "@/screens/startup-splash-screen";
import {
  type HostRuntimeBootstrapState,
  useHostRuntimeBootstrapState,
  useStoreReady,
} from "@/app/_layout";
import {
  buildHostRootRoute,
  mapPathnameToServer,
  parseHostWorkspaceRouteFromPathname,
  parseServerIdFromPathname,
} from "@/utils/host-routes";
import { shouldUseDesktopDaemon } from "@/desktop/daemon/desktop-daemon";

const WELCOME_ROUTE = "/welcome";

const isDesktop = shouldUseDesktopDaemon();

export default function Index() {
  const pathname = usePathname();
  const bootstrapState = useHostRuntimeBootstrapState();
  const storeReady = useStoreReady();
  const redirectRoute = resolveStartupRedirectRoute({
    bootstrapState,
    pathname,
    storeReady,
  });

  if (redirectRoute) {
    return <Redirect href={redirectRoute} />;
  }

  return <StartupSplashScreen bootstrapState={isDesktop ? bootstrapState : undefined} />;
}

function resolveStartupRedirectRoute(input: {
  bootstrapState: HostRuntimeBootstrapState;
  pathname: string;
  storeReady: boolean;
}): Href | null {
  const { bootstrapState, pathname, storeReady } = input;

  if (!storeReady || !bootstrapState.startupNavigation) {
    return null;
  }
  if (pathname !== "/" && pathname !== "") {
    return null;
  }

  const { target, lastRoute } = bootstrapState.startupNavigation;

  if (!target) {
    return WELCOME_ROUTE;
  }

  if (lastRoute) {
    return resolveLastRouteHref(lastRoute, target.serverId);
  }

  return buildHostRootRoute(target.serverId);
}

function resolveLastRouteHref(lastRoute: string, onlineServerId: string): Href {
  const lastServerId = parseServerIdFromPathname(lastRoute);
  if (!lastServerId || lastServerId === onlineServerId) {
    return lastRoute as Href;
  }
  // Another host's workspace is not meaningful on the online host.
  if (parseHostWorkspaceRouteFromPathname(lastRoute)) {
    return buildHostRootRoute(onlineServerId);
  }
  return mapPathnameToServer(lastRoute, onlineServerId);
}
