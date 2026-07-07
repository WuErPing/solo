import { useQuery } from "@tanstack/react-query";
import { useCallback, useMemo } from "react";
import type { LoopTemplateSummary, LoopListItem, LoopRecord } from "@server/server/loop/rpc-schemas";
import { useHostRuntimeClient, useHostRuntimeIsConnected } from "@/runtime/host-runtime";

export interface LoopTemplatesResult {
  templates: LoopTemplateSummary[];
  isLoading: boolean;
  isInitialLoad: boolean;
  isRevalidating: boolean;
  error: string | null;
  refreshAll: () => void;
}

export function loopTemplatesQueryKey(serverId: string | null): readonly string[] {
  return ["loop-templates", serverId ?? ""];
}

export function useLoopTemplates(options: {
  serverId?: string | null;
  enabled?: boolean;
}): LoopTemplatesResult {
  const serverId = useMemo(() => {
    const value = options.serverId;
    return typeof value === "string" && value.trim().length > 0 ? value.trim() : null;
  }, [options.serverId]);
  const enabled = options.enabled ?? true;
  const client = useHostRuntimeClient(serverId ?? "");
  const isConnected = useHostRuntimeIsConnected(serverId ?? "");
  const queryKey = useMemo(() => loopTemplatesQueryKey(serverId), [serverId]);

  const query = useQuery<{ templates: LoopTemplateSummary[]; error: string | null }, Error>({
    queryKey,
    enabled: Boolean(enabled && serverId && client && isConnected),
    staleTime: 30_000,
    queryFn: async () => {
      if (!client) {
        throw new Error("Daemon client not available");
      }
      const payload = await client.loopTemplateList();
      return {
        templates: payload.templates ?? [],
        error: payload.error ?? null,
      };
    },
  });

  const { data, isLoading, isFetching, refetch, error: queryError } = query;

  const refreshAll = useCallback(() => {
    if (!serverId || !client || !isConnected) {
      return;
    }
    void refetch();
  }, [client, isConnected, refetch, serverId]);

  const templates = data?.templates ?? [];
  const rpcError = data?.error ?? null;
  const isInitialLoad = isLoading && templates.length === 0;
  const isRevalidating = isFetching && !isLoading && templates.length > 0;

  return {
    templates,
    isLoading,
    isInitialLoad,
    isRevalidating,
    error: rpcError ?? (queryError ? queryError.message : null),
    refreshAll,
  };
}

export interface LoopTemplateDetailResult {
  template: LoopTemplateSummary | null;
  instances: LoopListItem[];
  latestRecord: LoopRecord | null;
  isLoading: boolean;
  isInitialLoad: boolean;
  isRevalidating: boolean;
  error: string | null;
  refreshAll: () => void;
}

export function loopTemplateDetailQueryKey(serverId: string | null, templateId: string | null): readonly string[] {
  return ["loop-template-detail", serverId ?? "", templateId ?? ""];
}

export function useLoopTemplateDetail(options: {
  serverId?: string | null;
  templateId?: string | null;
  enabled?: boolean;
}): LoopTemplateDetailResult {
  const serverId = useMemo(() => {
    const value = options.serverId;
    return typeof value === "string" && value.trim().length > 0 ? value.trim() : null;
  }, [options.serverId]);
  const templateId = useMemo(() => {
    const value = options.templateId;
    return typeof value === "string" && value.trim().length > 0 ? value.trim() : null;
  }, [options.templateId]);
  const enabled = options.enabled ?? true;
  const client = useHostRuntimeClient(serverId ?? "");
  const isConnected = useHostRuntimeIsConnected(serverId ?? "");
  const queryKey = useMemo(() => loopTemplateDetailQueryKey(serverId, templateId), [serverId, templateId]);

  const query = useQuery<{
    template: LoopTemplateSummary | null;
    instances: LoopListItem[];
    latestRecord: LoopRecord | null;
    error: string | null;
  }, Error>({
    queryKey,
    enabled: Boolean(enabled && serverId && templateId && client && isConnected),
    staleTime: 30_000,
    queryFn: async () => {
      if (!client || !templateId) {
        throw new Error("Daemon client not available");
      }
      const payload = await client.loopTemplateGet({ templateID: templateId });
      return {
        template: payload.template ?? null,
        instances: payload.instances ?? [],
        latestRecord: payload.latestRecord ?? null,
        error: payload.error ?? null,
      };
    },
  });

  const { data, isLoading, isFetching, refetch, error: queryError } = query;

  const refreshAll = useCallback(() => {
    if (!serverId || !templateId || !client || !isConnected) {
      return;
    }
    void refetch();
  }, [client, isConnected, refetch, serverId, templateId]);

  const template = data?.template ?? null;
  const instances = data?.instances ?? [];
  const latestRecord = data?.latestRecord ?? null;
  const rpcError = data?.error ?? null;
  const isInitialLoad = isLoading && template === null;
  const isRevalidating = isFetching && !isLoading && template !== null;

  return {
    template,
    instances,
    latestRecord,
    isLoading,
    isInitialLoad,
    isRevalidating,
    error: rpcError ?? (queryError ? queryError.message : null),
    refreshAll,
  };
}
