import { useMemo } from "react";

import { useSession } from "../data/session";
import type { Api } from "./clients";
import { createApi } from "./createApi";

// Clients for the session's server, rebuilt only when the server or token changes. A token the
// server rejects ends the session through expire.
export function useApi(): Api {
  const { serverUrl, token, expire } = useSession();
  return useMemo(() => createApi({ serverUrl, token, expire }), [serverUrl, token, expire]);
}
