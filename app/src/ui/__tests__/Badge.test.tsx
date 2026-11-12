import { fireEvent, screen } from "@testing-library/react-native";

import { Badge } from "../Badge";
import { renderThemed, snapshots } from "../test-utils";

describe("Badge", () => {
  snapshots(() => (
    <>
      <Badge label="Offline" icon="cloud-offline-outline" tone="warning" />
      <Badge label="Sync failed" tone="danger" />
    </>
  ));

  it("is a button when it has an action", async () => {
    const onPress = jest.fn();
    await renderThemed(<Badge label="Synced" onPress={onPress} />);
    await fireEvent.press(screen.getByRole("button", { name: "Synced" }));
    expect(onPress).toHaveBeenCalledTimes(1);
  });

  it("keeps a long status on one line", async () => {
    await renderThemed(<Badge label="Not synced: finalized, unknown program" />);
    expect(screen.getByText(/Not synced/)).toHaveProp("numberOfLines", 1);
  });
});
