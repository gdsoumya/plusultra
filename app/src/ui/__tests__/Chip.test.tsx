import { fireEvent, screen } from "@testing-library/react-native";

import { Chip } from "../Chip";
import { renderThemed, snapshots } from "../test-utils";

describe("Chip", () => {
  snapshots(() => (
    <>
      <Chip label="Barbell" selected />
      <Chip label="Dumbbell" icon="barbell-outline" />
    </>
  ));

  it("is a checkbox that toggles via onPress", async () => {
    const onPress = jest.fn();
    await renderThemed(<Chip label="Barbell" selected onPress={onPress} />);
    const chip = screen.getByRole("checkbox", { name: "Barbell" });
    expect(chip).toBeChecked();
    await fireEvent.press(chip);
    expect(onPress).toHaveBeenCalledTimes(1);
  });
});
