import { Code, ConnectError } from "@connectrpc/connect";

export const OFFLINE = "Could not reach the server. Check your connection and try again.";

// A failed fetch surfaces as Unknown with the TypeError as its cause.
export const isOffline = (e: unknown) =>
  e instanceof TypeError ||
  (e instanceof ConnectError &&
    (e.code === Code.Unavailable ||
      e.code === Code.DeadlineExceeded ||
      (e.code === Code.Unknown && e.cause instanceof TypeError)));

// What a screen shows for a failed call: offline for transport failures, else the server's reason.
export function errorMessage(e: unknown): string {
  if (isOffline(e)) return OFFLINE;
  if (e instanceof ConnectError) return e.rawMessage;
  return e instanceof Error ? e.message : String(e);
}
