import { useApi } from "../../../src/api/useApi";
import { useUnit } from "../../../src/data/settings";
import { useStore } from "../../../src/data/StoreProvider";
import { today } from "../../../src/domain/dates";
import ProgressScreen from "../../../src/features/progress/ProgressScreen";

export default function ProgressRoute() {
  return (
    <ProgressScreen
      insights={useApi().insights}
      store={useStore()}
      today={today()}
      unit={useUnit()}
    />
  );
}
