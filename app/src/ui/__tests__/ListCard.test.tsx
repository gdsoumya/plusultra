import { screen } from "@testing-library/react-native";

import { ListCard } from "../ListCard";
import { ListRow } from "../ListRow";
import { renderThemed } from "../test-utils";

it("puts a divider between rows and none around them", async () => {
  await renderThemed(
    <ListCard testID="card">
      <ListRow title="One" />
      <ListRow title="Two" />
      <ListRow title="Three" />
    </ListCard>,
  );
  const rows = screen.getByTestId("card").children;
  expect(rows).toHaveLength(5);
  expect(screen.getAllByLabelText(/One|Two|Three/)).toHaveLength(3);
});
