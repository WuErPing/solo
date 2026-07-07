import { useLocalSearchParams } from "expo-router";
import { HostRouteBootstrapBoundary } from "@/components/host-route-bootstrap-boundary";
import { LoopInstanceDetailScreen } from "@/screens/loop-instance-detail-screen";

export default function HostLoopInstanceDetailRoute() {
  return (
    <HostRouteBootstrapBoundary>
      <HostLoopInstanceDetailRouteContent />
    </HostRouteBootstrapBoundary>
  );
}

function HostLoopInstanceDetailRouteContent() {
  const params = useLocalSearchParams<{
    serverId?: string;
    loopId?: string;
    instanceId?: string;
  }>();
  const serverId = typeof params.serverId === "string" ? params.serverId : "";
  const loopId = typeof params.loopId === "string" ? params.loopId : "";
  const instanceId = typeof params.instanceId === "string" ? params.instanceId : "";

  return (
    <LoopInstanceDetailScreen
      serverId={serverId}
      instanceId={instanceId}
      templateId={loopId}
    />
  );
}
