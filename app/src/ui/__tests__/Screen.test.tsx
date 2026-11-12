import { fireEvent, screen, within } from "@testing-library/react-native";
import { Platform, StyleSheet } from "react-native";

import { Badge } from "../Badge";
import { Button } from "../Button";
import { Screen } from "../Screen";
import { Text } from "../Text";
import { renderThemed, snapshots } from "../test-utils";

let mockWide = false;
jest.mock("../useWide", () => ({ useWide: () => mockWide }));

describe("Screen", () => {
  snapshots(() => (
    <Screen title="Today" right={<Button title="Edit" variant="ghost" />} testID="screen">
      <Text>Content</Text>
    </Screen>
  ));

  it("keeps content in a centered column with a wider option", async () => {
    const column = async (wide: boolean) => {
      await renderThemed(
        <Screen title="Today" scroll={false} wide={wide}>
          <Text>Content</Text>
        </Screen>,
      );
      return screen.getByText("Content").parent!;
    };
    expect(await column(false)).toHaveStyle({ width: "100%", maxWidth: 752, alignSelf: "center" });
    expect(await column(true)).toHaveStyle({ maxWidth: 992 });
  });

  it("puts the title under the actions when there is a back action", async () => {
    await renderThemed(
      <Screen
        title="Wed Lower B deadlift"
        left={<Button title="Back" variant="ghost" />}
        right={<Button title="Finish" />}
        testID="screen"
      >
        <Text>Content</Text>
      </Screen>,
    );
    const actions = within(screen.getByTestId("screen-actions"));
    expect(actions.getByRole("button", { name: "Back" })).toBeOnTheScreen();
    expect(actions.getByRole("button", { name: "Finish" })).toBeOnTheScreen();
    expect(actions.queryByRole("header")).toBeNull();
    expect(screen.getByRole("header", { name: "Wed Lower B deadlift" })).toBeOnTheScreen();
  });

  describe("refresh", () => {
    const refresh = { refreshing: false, refresh: jest.fn() };
    const ui = (
      <Screen title="Today" right={<Button title="Edit" variant="ghost" />} refresh={refresh}>
        <Text>Content</Text>
      </Screen>
    );

    it("has no header button on native, where the pull gesture exists", async () => {
      await renderThemed(ui);
      expect(screen.queryByRole("button", { name: "Refresh" })).toBeNull();
      expect(refresh.refresh).not.toHaveBeenCalled();
    });

    it("offers a header button on a wide web window", async () => {
      const os = jest.replaceProperty(Platform, "OS", "web");
      mockWide = true;
      try {
        await renderThemed(ui);
        await fireEvent.press(screen.getByRole("button", { name: "Refresh" }));
        expect(refresh.refresh).toHaveBeenCalledTimes(1);
      } finally {
        os.restore();
        mockWide = false;
      }
    });

    it("leaves it out of a phone-width web header, which has no room for it", async () => {
      const os = jest.replaceProperty(Platform, "OS", "web");
      try {
        await renderThemed(ui);
        expect(screen.queryByRole("button", { name: "Refresh" })).toBeNull();
        expect(screen.getByRole("button", { name: "Edit" })).toBeOnTheScreen();
      } finally {
        os.restore();
      }
    });
  });

  it("centres every header control on one line, a status badge included", async () => {
    await renderThemed(
      <Screen
        title="Workout"
        left={<Button title="Back" variant="ghost" />}
        right={<Badge label="Synced" icon="checkmark-circle-outline" onPress={() => {}} />}
        testID="screen"
      >
        <Text>Content</Text>
      </Screen>,
    );
    expect(screen.getByTestId("screen-actions")).toHaveStyle({ alignItems: "center" });
    // A badge that aligns itself would sit at the top of the row instead.
    const badge = screen.getByRole("button", { name: "Synced" });
    expect(StyleSheet.flatten(badge.props.style).alignSelf).toBeUndefined();
  });

  it("keeps a long title on one line and never shrinks the actions", async () => {
    await renderThemed(
      <Screen
        title="A very long screen title that would otherwise wrap"
        right={
          <>
            <Badge label="Synced" icon="checkmark-circle-outline" />
            <Button title="New" icon="add" variant="secondary" compact />
          </>
        }
        testID="screen"
      >
        <Text>Content</Text>
      </Screen>,
    );
    const title = screen.getByRole("header");
    expect(title.props.numberOfLines).toBe(1);
    expect(title).toHaveStyle({ flexShrink: 1 });
    expect(screen.getByTestId("screen-actions")).toHaveStyle({ flexShrink: 0 });
  });

  it("renders without scrolling", async () => {
    await renderThemed(
      <Screen title="Today" scroll={false} testID="screen">
        <Text>Content</Text>
      </Screen>,
    );
    expect(screen.getByTestId("screen")).toBeOnTheScreen();
    expect(JSON.stringify(screen.toJSON())).not.toContain("RCTScrollView");
  });
});
