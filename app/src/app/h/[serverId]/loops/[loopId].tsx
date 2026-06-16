import { useLocalSearchParams } from "expo-router";
import { HostRouteBootstrapBoundary } from "@/components/host-route-bootstrap-boundary";
import { LoopDetailScreen } from "@/screens/loop-detail-screen";

export default function HostLoopDetailRoute() {
  return (
    <HostRouteBootstrapBoundary>
      <HostLoopDetailRouteContent />
    </HostRouteBootstrapBoundary>
  );
}

function HostLoopDetailRouteContent() {
  const params = useLocalSearchParams<{ serverId?: string; loopId?: string }>();
  const serverId = typeof params.serverId === "string" ? params.serverId : "";
  const loopId = typeof params.loopId === "string" ? params.loopId : "";

  return <LoopDetailScreen serverId={serverId} loopId={loopId} />;
}
