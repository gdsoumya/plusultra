import { useLocalSearchParams } from "expo-router";

import { useUnit } from "../../../../src/data/settings";
import ProgramEditorScreen from "../../../../src/features/programs/ProgramEditorScreen";

// Without an id the editor creates a new draft; kind=meal starts it from a meal script.
export default function EditProgramRoute() {
  const { id, kind } = useLocalSearchParams<{ id?: string; kind?: string }>();
  return (
    <ProgramEditorScreen id={id} unit={useUnit()} kind={kind === "meal" ? "meal" : "workout"} />
  );
}
