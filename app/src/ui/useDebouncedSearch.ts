import { useEffect } from "react";

// Runs `search(query)` once typing pauses; `search` must be stable (useCallback) or every render
// restarts the wait. The keyboard's Search key can still call it at once.
export function useDebouncedSearch(query: string, search: (query: string) => void, ms = 250) {
  useEffect(() => {
    if (!query.trim()) return;
    const timer = setTimeout(() => search(query), ms);
    return () => clearTimeout(timer);
  }, [query, search, ms]);
}
