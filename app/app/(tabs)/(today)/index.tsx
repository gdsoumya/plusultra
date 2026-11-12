import { useUser } from "../../../src/data/session";
import { useUnit } from "../../../src/data/settings";
import TodayScreen from "../../../src/features/today/TodayScreen";

export default function TodayRoute() {
  return <TodayScreen unit={useUnit()} userId={useUser()?.id ?? ""} />;
}
