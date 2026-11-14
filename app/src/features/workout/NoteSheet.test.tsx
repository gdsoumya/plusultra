import { fireEvent, screen } from "@testing-library/react-native";

import { renderThemed } from "../../ui/test-utils";
import { NoteSheet } from "./NoteSheet";

describe("NoteSheet", () => {
  it("starts empty with Save off and no Remove", async () => {
    const onSave = jest.fn();
    await renderThemed(<NoteSheet notes="" onSave={onSave} onClose={jest.fn()} />);
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Remove note" })).toBeNull();
    await fireEvent.changeText(screen.getByLabelText("Note"), "Felt heavy");
    await fireEvent.press(screen.getByRole("button", { name: "Save" }));
    expect(onSave).toHaveBeenCalledWith("Felt heavy");
  });

  it("edits the existing note and can remove it", async () => {
    const onSave = jest.fn();
    await renderThemed(<NoteSheet notes="Felt heavy" onSave={onSave} onClose={jest.fn()} />);
    expect(screen.getByLabelText("Note").props.value).toBe("Felt heavy");
    await fireEvent.press(screen.getByRole("button", { name: "Remove note" }));
    expect(onSave).toHaveBeenCalledWith("");
  });
});
