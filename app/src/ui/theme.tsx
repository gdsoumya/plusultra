import { createContext, useContext, useMemo, type ReactNode } from "react";

import { colors, type Colors } from "./tokens";

export type Theme = {
  colors: Colors;
  fontScale: number;
};

const ThemeContext = createContext<Theme>({ colors, fontScale: 1 });

type ThemeProviderProps = {
  /** Multiplies every type size; simulates system text scaling in the gallery and tests. */
  fontScale?: number;
  children: ReactNode;
};

// The app is dark only; the provider carries the palette and the text scale.
export function ThemeProvider({ fontScale = 1, children }: ThemeProviderProps) {
  const theme = useMemo(() => ({ colors, fontScale }), [fontScale]);
  return <ThemeContext.Provider value={theme}>{children}</ThemeContext.Provider>;
}

export const useTheme = () => useContext(ThemeContext);
