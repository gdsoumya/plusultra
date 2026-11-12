import { screen } from "@testing-library/react-native";

import { Field } from "../Field";
import { renderThemed, snapshots } from "../test-utils";

describe("Field", () => {
  snapshots(() => (
    <>
      <Field label="Server URL" value="" placeholder="https://" helper="Your self-hosted server" />
      <Field label="Weight" value="102.5" numeric />
      <Field label="Reps" value="x" numeric error="Enter a number" />
    </>
  ));

  it("shows the error instead of the helper", async () => {
    await renderThemed(<Field label="Reps" value="x" helper="Per set" error="Enter a number" />);
    expect(screen.getByText("Enter a number")).toBeOnTheScreen();
    expect(screen.queryByText("Per set")).toBeNull();
  });

  it("links the error, or else the helper, to the input as its hint", async () => {
    await renderThemed(<Field label="Reps" value="x" helper="Per set" error="Enter a number" />);
    expect(screen.getByLabelText("Reps")).toHaveProp("accessibilityHint", "Enter a number");
    await renderThemed(<Field label="Sets" value="3" helper="Per exercise" />);
    expect(screen.getByLabelText("Sets")).toHaveProp("accessibilityHint", "Per exercise");
  });

  it("uses a decimal keyboard in numeric mode", async () => {
    await renderThemed(<Field label="Weight" value="" numeric />);
    expect(screen.getByLabelText("Weight")).toHaveProp("keyboardType", "decimal-pad");
  });
});
