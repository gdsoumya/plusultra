import { fireEvent, screen } from "@testing-library/react-native";

import { ListRow } from "../ListRow";
import { renderThemed, snapshots } from "../test-utils";

describe("ListRow", () => {
  snapshots(() => (
    <>
      <ListRow
        icon="barbell"
        title="Squat"
        subtitle="3 x 5 at 100 kg"
        value="Done"
        onPress={() => {}}
      />
      <ListRow title="Units" value="kg" />
    </>
  ));

  it("reads title, subtitle and value as one label", async () => {
    await renderThemed(
      <ListRow title="Squat" subtitle="3 x 5 at 100 kg" value="Done" onPress={() => {}} />,
    );
    expect(screen.getByRole("button", { name: "Squat, 3 x 5 at 100 kg, Done" })).toBeOnTheScreen();
  });

  it("gives static rows a label but no role", async () => {
    await renderThemed(<ListRow title="Units" value="kg" />);
    expect(screen.getByLabelText("Units, kg")).toBeOnTheScreen();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("fires onPress", async () => {
    const onPress = jest.fn();
    await renderThemed(<ListRow title="Squat" onPress={onPress} />);
    await fireEvent.press(screen.getByRole("button", { name: "Squat" }));
    expect(onPress).toHaveBeenCalledTimes(1);
  });
});
