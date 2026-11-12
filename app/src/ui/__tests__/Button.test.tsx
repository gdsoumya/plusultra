import { fireEvent, screen } from "@testing-library/react-native";

import { Button } from "../Button";
import { renderThemed, snapshots } from "../test-utils";

describe("Button", () => {
  snapshots(() => (
    <>
      <Button title="Start workout" icon="play" />
      <Button title="Skip" variant="secondary" />
      <Button title="Details" variant="ghost" />
      <Button title="Delete" variant="danger" />
      <Button title="Saving" loading />
      <Button title="Disabled" disabled />
    </>
  ));

  it("lets long titles wrap instead of clipping at large text sizes", async () => {
    await renderThemed(<Button title="Finish workout and save" />, 2);
    expect(screen.getByText("Finish workout and save")).not.toHaveProp("numberOfLines");
  });

  it("is named by its title alone, not the icon glyph", async () => {
    await renderThemed(<Button title="Start" icon="play" />);
    expect(screen.getByRole("button", { name: "Start" })).toBeOnTheScreen();
  });

  it("drops the label when compact but keeps it as the name", async () => {
    await renderThemed(<Button title="New" icon="add" compact />);
    expect(screen.getByRole("button", { name: "New" })).toBeOnTheScreen();
    expect(screen.queryByText("New")).toBeNull();
  });

  it("fires onPress", async () => {
    const onPress = jest.fn();
    await renderThemed(<Button title="Save" onPress={onPress} />);
    await fireEvent.press(screen.getByRole("button", { name: "Save" }));
    expect(onPress).toHaveBeenCalledTimes(1);
  });

  it.each([{ disabled: true }, { loading: true }])("does not fire when %o", async (props) => {
    const onPress = jest.fn();
    await renderThemed(<Button title="Save" onPress={onPress} {...props} />);
    await fireEvent.press(screen.getByRole("button", { name: "Save" }));
    expect(onPress).not.toHaveBeenCalled();
  });
});
