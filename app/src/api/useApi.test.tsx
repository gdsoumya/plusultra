import { renderHook } from "@testing-library/react-native";

import { createApi } from "./createApi";
import { useApi } from "./useApi";

const mockSession = { serverUrl: "https://lift.example.com", token: "token-1", expire: jest.fn() };
jest.mock("../data/session", () => ({ useSession: () => mockSession }));
jest.mock("./createApi", () => ({ createApi: jest.fn(() => ({})) }));

it("builds the clients with the session's expiry hook, once per session", async () => {
  const { result, rerender } = await renderHook(() => useApi());
  await rerender({});
  expect(createApi).toHaveBeenCalledTimes(1);
  expect(createApi).toHaveBeenCalledWith({
    serverUrl: "https://lift.example.com",
    token: "token-1",
    expire: mockSession.expire,
  });
  expect(result.current).toBe(jest.mocked(createApi).mock.results[0].value);
});
