import { fireEvent, screen, within } from "@testing-library/react-native";

import { renderThemed } from "../../ui/test-utils";
import { RangePicker, rangeFor } from "./RangePicker";
import { rangeOf } from "./shared";

const today = "2026-11-11";
const range = rangeOf("2026-10-14", today);

async function open(choice: Parameters<typeof RangePicker>[0]["choice"] = { days: 28 }) {
  const onChange = jest.fn();
  await renderThemed(<RangePicker range={range} choice={choice} onChange={onChange} />);
  await fireEvent.press(screen.getByTestId("progress-range"));
  return { onChange, sheet: within(await screen.findByTestId("progress-range-sheet")) };
}

describe("RangePicker", () => {
  it("counts a preset back from today", () => {
    expect(rangeFor({ days: 7 }, today)).toEqual(rangeOf("2026-11-05", today));
    expect(rangeFor({ from: "2026-10-01", to: "2026-10-15" }, today).days).toBe(14);
  });

  it("picks a preset", async () => {
    const { onChange, sheet } = await open();
    await fireEvent.press(sheet.getByLabelText("3 months"));
    expect(onChange).toHaveBeenCalledWith({ days: 91 });
  });

  it("picks a custom span from the calendar: first tap From, second tap To", async () => {
    const { onChange, sheet } = await open();
    await fireEvent.press(sheet.getByLabelText("Custom"));
    expect(sheet.getByText("November 2026")).toBeOnTheScreen();
    expect(sheet.getByRole("button", { name: "Apply" })).toBeDisabled();
    await fireEvent.press(sheet.getByRole("button", { name: "Previous month" }));
    expect(sheet.getByText("October 2026")).toBeOnTheScreen();
    await fireEvent.press(sheet.getByRole("button", { name: "Thursday 1 October" }));
    expect(sheet.getByText(/^From (Oct 1|1 Oct) – To …$/)).toBeOnTheScreen();
    expect(sheet.getByRole("button", { name: "Apply" })).toBeDisabled();
    await fireEvent.press(sheet.getByRole("button", { name: "Thursday 15 October" }));
    expect(sheet.getByText(/^From (Oct 1|1 Oct) – To (Oct 15|15 Oct)$/)).toBeOnTheScreen();
    expect(sheet.getByRole("button", { name: "Thursday 8 October" })).toBeSelected();
    expect(sheet.getByRole("button", { name: "Friday 16 October" })).not.toBeSelected();
    await fireEvent.press(sheet.getByRole("button", { name: "Apply" }));
    expect(onChange).toHaveBeenCalledWith({ from: "2026-10-01", to: "2026-10-15" });
  });

  it("restarts the span on a tap before From", async () => {
    const { sheet } = await open();
    await fireEvent.press(sheet.getByLabelText("Custom"));
    await fireEvent.press(sheet.getByRole("button", { name: "Previous month" }));
    await fireEvent.press(sheet.getByRole("button", { name: "Thursday 15 October" }));
    await fireEvent.press(sheet.getByRole("button", { name: "Thursday 1 October" }));
    expect(sheet.getByText(/^From (Oct 1|1 Oct) – To …$/)).toBeOnTheScreen();
    expect(sheet.getByRole("button", { name: "Apply" })).toBeDisabled();
  });

  it("opens on the remembered custom span", async () => {
    const { sheet } = await open({ from: "2026-10-01", to: "2026-10-15" });
    expect(screen.getByText(/Oct 1 – Oct 15|1 Oct – 15 Oct/)).toBeOnTheScreen();
    expect(sheet.getByLabelText("Custom")).toBeChecked();
    expect(sheet.getByText("October 2026")).toBeOnTheScreen();
    expect(sheet.getByRole("button", { name: "Thursday 15 October" })).toBeSelected();
    expect(sheet.getByRole("button", { name: "Apply" })).toBeEnabled();
  });
});
