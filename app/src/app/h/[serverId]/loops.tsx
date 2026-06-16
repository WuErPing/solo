import { useLocalSearchParams } from "expo-router";
import { HostRouteBootstrapBoundary } from "@/components/host-route-bootstrap-boundary";
import { LoopsScreen } from "@/screens/loops-screen";

export default function HostLoopsRoute() {
  return (
    <HostRouteBootstrapBoundary>
      <HostLoopsRouteContent />
    </HostRouteBootstrapBoundary>
  );
}

function HostLoopsRouteContent() {
  const params = useLocalSearchParams<{ serverId?: string }>();
  const serverId = typeof params.serverId === "string" ? params.serverId : "";

  return <LoopsScreen serverId={serverId} />;
}
