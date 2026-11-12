import { useLocalSearchParams } from "expo-router";

import { useUser } from "../../../../../src/data/session";
import { useUnit } from "../../../../../src/data/settings";
import { today } from "../../../../../src/domain/dates";
import DayScreen from "../../../../../src/features/calendar/DayScreen";

export default function DayRoute() {
  const { date } = useLocalSearchParams<{ date: string }>();
  return <DayScreen date={date} today={today()} unit={useUnit()} userId={useUser()?.id ?? ""} />;
}
