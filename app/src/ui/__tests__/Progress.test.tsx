import { screen } from "@testing-library/react-native";

import { Numeral } from "../Numeral";
import { Progress } from "../Progress";
import { renderThemed, snapshots } from "../test-utils";

describe("Progress", () => {
  snapshots(() => (
    <>
      <Progress value={0.4} />
      <Progress value={0.75} variant="ring">
        <Numeral value="3/4" />
      </Progress>
    </>
  ));

  it.each([
    { value: 1.7, now: 100 },
    { value: -0.5, now: 0 },
    { value: 0.42, now: 42 },
    { value: Number.NaN, now: 0 },
  ])("clamps $value to $now%", async ({ value, now }) => {
    await renderThemed(
      <>
        <Progress value={value} />
        <Progress value={value} variant="ring" />
      </>,
    );
    for (const bar of screen.getAllByRole("progressbar")) {
      expect(bar).toHaveAccessibilityValue({ min: 0, max: 100, now });
    }
  });
});
