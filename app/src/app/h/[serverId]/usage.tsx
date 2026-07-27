import { useLocalSearchParams } from "expo-router";
import { HostRouteBootstrapBoundary } from "@/components/host-route-bootstrap-boundary";
import { UsageScreen } from "@/screens/usage/usage-screen";

export default function HostUsageRoute() {
  return (
    <HostRouteBootstrapBoundary>
      <HostUsageRouteContent />
    </HostRouteBootstrapBoundary>
  );
}

function HostUsageRouteContent() {
  const params = useLocalSearchParams<{ serverId?: string }>();
  const serverId = typeof params.serverId === "string" ? params.serverId : "";

  return <UsageScreen serverId={serverId} />;
}
