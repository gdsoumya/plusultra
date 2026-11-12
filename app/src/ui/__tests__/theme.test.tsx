import { render, screen } from "@testing-library/react-native";
import * as RN from "react-native";

import { ThemeProvider, useTheme } from "../theme";
import { colors } from "../tokens";

function Probe() {
  const { colors, fontScale } = useTheme();
  return <RN.Text testID="probe">{`${colors.bg} ${fontScale}`}</RN.Text>;
}

it("is dark whatever the system scheme says", async () => {
  jest.spyOn(RN, "useColorScheme").mockReturnValue("light");
  await render(
    <ThemeProvider>
      <Probe />
    </ThemeProvider>,
  );
  expect(screen.getByTestId("probe")).toHaveTextContent(`${colors.bg} 1`);
});

it("takes the font scale from props", async () => {
  await render(
    <ThemeProvider fontScale={1.3}>
      <Probe />
    </ThemeProvider>,
  );
  expect(screen.getByTestId("probe")).toHaveTextContent(`${colors.bg} 1.3`);
});

it("has the palette without a provider", async () => {
  await render(<Probe />);
  expect(screen.getByTestId("probe")).toHaveTextContent(`${colors.bg} 1`);
});
