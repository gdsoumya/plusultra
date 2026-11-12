import { useLocalSearchParams } from "expo-router";

import { useUnit } from "../../../../src/data/settings";
import ProgramEditorScreen from "../../../../src/features/programs/ProgramEditorScreen";

// Without an id the editor creates a new draft.
export default function EditProgramRoute() {
  const { id } = useLocalSearchParams<{ id?: string }>();
  return <ProgramEditorScreen id={id} unit={useUnit()} />;
}
