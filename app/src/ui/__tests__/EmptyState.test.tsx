import { fireEvent, screen } from "@testing-library/react-native";

import { EmptyState } from "../EmptyState";
import { renderThemed, snapshots } from "../test-utils";

describe("EmptyState", () => {
  snapshots(() => (
    <EmptyState
      icon="calendar-outline"
      title="No program yet"
      message="Pick a program to see your plan here."
      action={{ title: "Browse programs", onPress: () => {} }}
    />
  ));

  it("fires its action", async () => {
    const onPress = jest.fn();
    await renderThemed(
      <EmptyState icon="cafe-outline" title="Rest day" action={{ title: "Log anyway", onPress }} />,
    );
    await fireEvent.press(screen.getByRole("button", { name: "Log anyway" }));
    expect(onPress).toHaveBeenCalledTimes(1);
  });
});
