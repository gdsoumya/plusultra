import { act, render, screen } from "@testing-library/react-native";
import type { ReactElement } from "react";
import { RefreshControl } from "react-native";

import { ThemeProvider } from "./theme";

export function renderThemed(ui: ReactElement, fontScale = 1) {
  return render(<ThemeProvider fontScale={fontScale}>{ui}</ThemeProvider>);
}

// Pulls the mounted Screen to refresh and waits for the control to clear. React Native's jest
// mock keeps the mounted RefreshControl in a static ref.
export async function pullToRefresh() {
  const control = (RefreshControl as unknown as { latestRef: { props: RefreshControlProps } })
    .latestRef;
  await act(() => control.props.onRefresh());
  expect(control.props.refreshing).toBe(false);
}
type RefreshControlProps = { refreshing: boolean; onRefresh(): Promise<void> };

// Snapshots `ui` at text scales 1.0, 1.3 and 2.0, after animations settle.
export function snapshots(ui: () => ReactElement) {
  afterEach(() => jest.useRealTimers());
  it.each([1, 1.3, 2])("matches snapshot (%s x)", async (fontScale) => {
    jest.useFakeTimers();
    await renderThemed(ui(), fontScale);
    await act(() => jest.advanceTimersByTime(1000));
    expect(screen.toJSON()).toMatchSnapshot();
  });
}
