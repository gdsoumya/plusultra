import { render, screen } from "@testing-library/react-native";
import { SafeAreaInsetsContext } from "react-native-safe-area-context";

import { maxWidth, space, ThemeProvider } from "../../ui";
import { TimerBar } from "./TimerBar";

describe("TimerBar", () => {
  it("sits on the tab bar without its own room for the home indicator", async () => {
    await render(
      <ThemeProvider>
        <SafeAreaInsetsContext.Provider value={{ top: 0, left: 0, right: 0, bottom: 34 }}>
          <TimerBar
            icon="timer-outline"
            title="Rest"
            value="0:30"
            action="Skip"
            onAction={() => {}}
            onAdjust={() => {}}
            testID="rest-timer"
          />
        </SafeAreaInsetsContext.Provider>
      </ThemeProvider>,
    );
    expect(screen.getByTestId("rest-timer-row")).toHaveStyle({ paddingBottom: 8 });
  });

  it("lines its content up with the screen's centered column on a wide window", async () => {
    await render(
      <ThemeProvider>
        <TimerBar
          icon="timer-outline"
          title="Rest"
          value="0:30"
          action="Skip"
          onAction={() => {}}
          testID="rest-timer"
        />
      </ThemeProvider>,
    );
    expect(screen.getByTestId("rest-timer-row")).toHaveStyle({
      width: "100%",
      maxWidth: maxWidth.content + 2 * space[4],
      alignSelf: "center",
    });
  });
});
