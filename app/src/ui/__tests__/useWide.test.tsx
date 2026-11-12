import { renderHook } from "@testing-library/react-native";
import { Dimensions } from "react-native";

import { useWide } from "../useWide";

const window = (width: number) =>
  jest.spyOn(Dimensions, "get").mockReturnValue({ width, height: 800, scale: 1, fontScale: 1 });

afterEach(() => jest.restoreAllMocks());

it("is wide from 900 px", async () => {
  window(899);
  expect((await renderHook(() => useWide())).result.current).toBe(false);
  window(900);
  expect((await renderHook(() => useWide())).result.current).toBe(true);
});
