import { Code, ConnectError } from "@connectrpc/connect";

import { ValidateProgramResponseSchema, type ScriptError } from "../../gen/plusultra/v1/program_pb";

// Line errors a rejected create or update carries as a ValidateProgramResponse detail.
export const scriptErrorsOf = (e: unknown): ScriptError[] | undefined =>
  e instanceof ConnectError && e.code === Code.InvalidArgument
    ? e.findDetails(ValidateProgramResponseSchema)[0]?.errors
    : undefined;
