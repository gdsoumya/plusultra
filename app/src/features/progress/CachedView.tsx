import type { ReactNode } from "react";

import { formatDateTime } from "../../domain/dates";
import { EmptyState, Loading, Text } from "../../ui";
import type { Cached } from "./useCached";

// Renders cached-or-fresh data, a loading line, or the error with a retry.
export function CachedView<T>({
  state: { data, at, error, loading, retry },
  children,
}: {
  state: Cached<T>;
  children: (data: T) => ReactNode;
}) {
  if (!data) {
    if (error) {
      return (
        <EmptyState
          icon="cloud-offline-outline"
          title="Couldn't load"
          message={error}
          action={{ title: "Retry", onPress: retry }}
        />
      );
    }
    return loading ? <Loading /> : null;
  }
  return (
    <>
      {error && at && (
        <Text variant="caption">Offline · last updated {formatDateTime(new Date(at))}</Text>
      )}
      {children(data)}
    </>
  );
}
