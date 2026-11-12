import { useState } from "react";

import { errorMessage } from "../../api/errors";

// Runs a form's action and keeps the message of its failure for the screen.
export function useSubmit() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const submit = async (action: () => Promise<void>) => {
    setBusy(true);
    setError(undefined);
    try {
      await action();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  return { busy, error, submit };
}
