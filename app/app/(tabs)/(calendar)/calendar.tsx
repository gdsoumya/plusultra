import { today } from "../../../src/domain/dates";
import CalendarScreen from "../../../src/features/calendar/CalendarScreen";

export default function CalendarRoute() {
  return <CalendarScreen today={today()} />;
}
