import { useWindowDimensions } from "react-native";

import { wideBreakpoint } from "./tokens";

// True on a desktop-sized window, where the tabs move to a top bar over the content column.
export const useWide = () => useWindowDimensions().width >= wideBreakpoint;
