import { useLocalSearchParams } from "expo-router";
import { HostRouteBootstrapBoundary } from "@/components/host-route-bootstrap-boundary";
import { ScheduleDetailScreen } from "@/screens/schedule-detail-screen";

export default function HostScheduleDetailRoute() {
  return (
    <HostRouteBootstrapBoundary>
      <HostScheduleDetailRouteContent />
    </HostRouteBootstrapBoundary>
  );
}

function HostScheduleDetailRouteContent() {
  const params = useLocalSearchParams<{ serverId?: string; scheduleId?: string }>();
  const serverId = typeof params.serverId === "string" ? params.serverId : "";
  const scheduleId = typeof params.scheduleId === "string" ? params.scheduleId : "";

  return <ScheduleDetailScreen serverId={serverId} scheduleId={scheduleId} />;
}
