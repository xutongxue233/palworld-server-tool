import {
  Fragment,
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useQuery } from "@tanstack/react-query";

import {
  api,
  getServerScope,
  LOCAL_SERVER_SCOPE,
  setServerScope,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import type { FleetNode, FleetStatus } from "@/types/api";

interface FleetContextValue {
  scope: string;
  activeNode: FleetNode | null;
  nodes: FleetNode[];
  issues: FleetStatus["issues"];
  isFetching: boolean;
  error: Error | null;
  selectNode: (scope: string) => boolean;
  refetch: () => Promise<unknown>;
}

const FleetContext = createContext<FleetContextValue | null>(null);

export const fleetQueryKey = ["fleet", "nodes"] as const;

export function FleetProvider({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth();
  const [requestedScope, setRequestedScope] = useState(LOCAL_SERVER_SCOPE);
  const fleetQuery = useQuery({
    queryKey: fleetQueryKey,
    queryFn: api.getFleetNodes,
    enabled: isAuthenticated,
    refetchInterval: 10_000,
    retry: 1,
  });

  const nodes = useMemo(
    () => fleetQuery.data?.nodes ?? [],
    [fleetQuery.data?.nodes],
  );
  const requestedNodeExists =
    requestedScope === LOCAL_SERVER_SCOPE ||
    nodes.some((node) => node.scope === requestedScope);
  const scope =
    isAuthenticated && requestedNodeExists
      ? requestedScope
      : LOCAL_SERVER_SCOPE;
  if (getServerScope() !== scope) {
    setServerScope(scope);
  }
  const activeNode =
    nodes.find((node) => node.scope === scope) ??
    nodes.find((node) => node.scope === LOCAL_SERVER_SCOPE) ??
    null;

  const applyScope = useCallback((nextScope: string) => {
    setServerScope(nextScope);
    setRequestedScope(nextScope);
  }, []);

  const selectNode = useCallback(
    (nextScope: string) => {
      const target = nodes.find((node) => node.scope === nextScope);
      if (!target || (!target.selectable && target.scope !== scope)) {
        return false;
      }
      applyScope(target.scope);
      return true;
    },
    [applyScope, nodes, scope],
  );

  useEffect(() => {
    const reset = () => {
      applyScope(LOCAL_SERVER_SCOPE);
    };
    window.addEventListener("palworld:fleet-reset", reset);
    return () => window.removeEventListener("palworld:fleet-reset", reset);
  }, [applyScope]);

  const value = useMemo<FleetContextValue>(
    () => ({
      scope,
      activeNode,
      nodes,
      issues: fleetQuery.data?.issues,
      isFetching: fleetQuery.isFetching,
      error: fleetQuery.error,
      selectNode,
      refetch: fleetQuery.refetch,
    }),
    [
      activeNode,
      fleetQuery.data?.issues,
      fleetQuery.error,
      fleetQuery.isFetching,
      fleetQuery.refetch,
      nodes,
      scope,
      selectNode,
    ],
  );

  return (
    <FleetContext.Provider value={value}>
      <Fragment key={scope}>{children}</Fragment>
    </FleetContext.Provider>
  );
}

export function useFleet() {
  const value = useContext(FleetContext);
  if (!value) throw new Error("useFleet must be used inside FleetProvider");
  return value;
}
