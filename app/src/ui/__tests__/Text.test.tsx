import { screen } from "@testing-library/react-native";

import { Numeral } from "../Numeral";
import { Text } from "../Text";
import { renderThemed, snapshots } from "../test-utils";

describe("Text", () => {
  snapshots(() => (
    <>
      <Text variant="title">Title</Text>
      <Text variant="heading">Heading</Text>
      <Text>Body</Text>
      <Text variant="label">Label</Text>
      <Text variant="caption">Caption</Text>
      <Text color="accent">Accent body</Text>
    </>
  ));

  it("scales font size and line height with the theme font scale", async () => {
    await renderThemed(<Text>Body</Text>, 2);
    expect(screen.getByText("Body")).toHaveStyle({ fontSize: 32, lineHeight: 44 });
  });
});

describe("Numeral", () => {
  snapshots(() => (
    <>
      <Numeral value={102.5} />
      <Numeral value="1,240" size="lg" color="accent" />
    </>
  ));

  it("uses tabular figures", async () => {
    await renderThemed(<Numeral value={80} />);
    expect(screen.getByText("80")).toHaveStyle({ fontVariant: ["tabular-nums"], fontSize: 28 });
  });
});
