import { screen } from "@testing-library/react-native";

import { renderThemed } from "../../ui/test-utils";
import { WeekPlan } from "./WeekPlan";

jest.mock("expo-router", () => ({ useRouter: () => ({ push: jest.fn() }) }));

const days = [
  { week: 1, day: 1, name: "Mon conditioning", kind: "conditioning" },
  { week: 1, day: 2, name: "Day 2", kind: "rest" },
  { week: 1, day: 3, name: "Upper", kind: "training" },
];

it("shows a day's kind only when its name does not already say it", async () => {
  await renderThemed(
    <WeekPlan
      programId="p"
      unit="kg"
      weeks={1}
      days={days}
      plans={[]}
      week={1}
      onWeek={() => {}}
    />,
  );
  expect(screen.getByText("1. Mon conditioning")).toBeOnTheScreen();
  expect(screen.queryByText("conditioning")).toBeNull();
  expect(screen.getByText("rest")).toBeOnTheScreen();
  expect(screen.queryByText("training")).toBeNull();
});
