import type { SidebarStateBucket } from "@/utils/sidebar-agent-state";

export type StatusLoaderBucket = SidebarStateBucket;

export function shouldRenderSyncedStatusLoader(input: {
  bucket: StatusLoaderBucket | null | undefined;
}): boolean {
  return input.bucket === "running";
}
