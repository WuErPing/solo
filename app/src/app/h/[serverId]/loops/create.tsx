import { useLocalSearchParams } from "expo-router";
import { HostRouteBootstrapBoundary } from "@/components/host-route-bootstrap-boundary";
import { LoopCreateScreen } from "@/screens/loop-create-screen";

export default function HostLoopCreateRoute() {
  return (
    <HostRouteBootstrapBoundary>
      <HostLoopCreateRouteContent />
    </HostRouteBootstrapBoundary>
  );
}

function HostLoopCreateRouteContent() {
  const params = useLocalSearchParams<{ serverId?: string }>();
  const serverId = typeof params.serverId === "string" ? params.serverId : "";

  return <LoopCreateScreen serverId={serverId} />;
}
