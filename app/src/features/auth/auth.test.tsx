import { toJson } from "@bufbuild/protobuf";
import { fireEvent, renderHook, renderRouter, screen, waitFor } from "expo-router/testing-library";
import * as SecureStore from "expo-secure-store";
import { Platform } from "react-native";

import { inviteCode, user } from "../../api/fixtures/user";
import { MOCK_SERVER } from "../../data/session";
import { useStore } from "../../data/StoreProvider";
import { dayLog } from "../../data/test-utils";
import { UserSchema } from "../../gen/plusultra/v1/auth_pb";

jest.mock("expo-secure-store", () => ({ getItemAsync: jest.fn(), setItemAsync: jest.fn() }));

const KEY = "plusultra.session";
const stored = new Map<string, string>();
const fetch = jest.spyOn(globalThis, "fetch");

beforeEach(() => {
  stored.clear();
  fetch.mockReset();
  jest.mocked(SecureStore.getItemAsync).mockImplementation(async (k) => stored.get(k) ?? null);
  jest.mocked(SecureStore.setItemAsync).mockImplementation(async (k, v) => void stored.set(k, v));
});

const offline = () => fetch.mockRejectedValue(new TypeError("Network request failed"));

// The hosted web build: Platform says web and the page has an origin.
let platform: ReturnType<typeof jest.replaceProperty> | undefined;
afterEach(() => platform?.restore());
function onWeb(origin: string) {
  platform = jest.replaceProperty(Platform, "OS", "web");
  Object.defineProperty(globalThis, "location", { value: { origin }, configurable: true });
}

function storeSession(serverUrl: string, signedIn = false) {
  const session = signedIn
    ? { serverUrl, token: "token-1", user: toJson(UserSchema, user("2026-11-11")) }
    : { serverUrl };
  stored.set(KEY, JSON.stringify(session));
}

const renderApp = (initialUrl = "/", overrides = {}) =>
  renderRouter({ appDir: "./app", overrides }, { initialUrl });

const at = (pathname: string) => waitFor(() => expect(screen).toHavePathname(pathname));
const onToday = () => waitFor(() => expect(screen.getByTestId("today-screen")).toBeOnTheScreen());
const type = (label: string, text: string) =>
  fireEvent.changeText(screen.getByLabelText(label), text);
const press = (name: string) => fireEvent.press(screen.getByRole("button", { name }));

describe("auth gate", () => {
  it("starts on the server screen without a session", async () => {
    await renderApp();
    await at("/server");
    expect(screen.getByRole("button", { name: "Try the demo" })).toBeOnTheScreen();
  });

  it("starts on Today with a session", async () => {
    storeSession(MOCK_SERVER, true);
    await renderApp();
    await onToday();
    expect(screen).toHavePathname("/");
  });

  it("keeps a signed-in user out of the auth screens", async () => {
    storeSession(MOCK_SERVER, true);
    await renderApp("/login");
    await onToday();
  });

  it("keeps a signed-out deep link out of the app", async () => {
    storeSession(MOCK_SERVER);
    await renderApp("/workout/workout-2026-11-11");
    await at("/server");
    expect(screen.queryByTestId("workout-screen")).toBeNull();
  });

  it("sends an expired session back to login with a message", async () => {
    storeSession("https://lift.example.com", true);
    // A fresh Response per call: the sync engine's startup pull reads a body too.
    fetch.mockImplementation(
      async () =>
        new Response(JSON.stringify({ code: "unauthenticated", message: "session expired" }), {
          status: 401,
          headers: { "Content-Type": "application/json" },
        }),
    );
    // The sync engine's startup pull is the first call the server rejects.
    await renderApp("/");
    await at("/login");
    expect(screen.getByText(/Session expired/)).toBeOnTheScreen();
    expect(JSON.parse(stored.get(KEY)!)).toEqual({
      serverUrl: "https://lift.example.com",
      expired: true,
    });
  });
});

describe("server screen", () => {
  const connect = async (url: string) => {
    await type("Server address", url);
    await press("Connect");
  };

  it("rejects a malformed address before probing", async () => {
    await renderApp();
    await at("/server");
    await connect("lift.example.com");
    await waitFor(() => expect(screen.getByText(/http:\/\/ or https:\/\//)).toBeOnTheScreen());
    expect(fetch).not.toHaveBeenCalled();
  });

  it("shows why the server could not be reached and offers a retry", async () => {
    offline();
    await renderApp();
    await at("/server");
    await connect("https://nope.example.com");
    await waitFor(() => expect(screen.getByText(/Could not reach the server/)).toBeOnTheScreen());
    expect(fetch).toHaveBeenCalledWith("https://nope.example.com/healthz", expect.anything());
    await press("Retry");
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
    expect(screen).toHavePathname("/server");
  });

  it("moves to login once the server answers", async () => {
    fetch.mockResolvedValue(new Response("ok dev"));
    await renderApp();
    await at("/server");
    await connect("https://lift.example.com/");
    await at("/login");
    expect(screen.getByText(/lift\.example\.com/)).toBeOnTheScreen();
    expect(JSON.parse(stored.get(KEY)!)).toEqual({ serverUrl: "https://lift.example.com" });
  });

  it("drops the previous server's rows when connecting elsewhere", async () => {
    fetch.mockResolvedValue(new Response("ok dev"));
    storeSession("https://old.example.com");
    const store = (await renderHook(() => useStore())).result.current;
    store.put("dayLogs", dayLog("a", "2026-11-10"));
    await renderApp();
    await at("/server");
    await connect("https://new.example.com");
    await at("/login");
    expect(store.list("dayLogs")).toEqual([]);
  });

  it("connects to the page's own origin on the hosted web build", async () => {
    onWeb("https://lift.example.com");
    fetch.mockResolvedValue(new Response("ok dev"));
    await renderApp();
    await at("/login");
    expect(fetch).toHaveBeenCalledWith("https://lift.example.com/healthz", expect.anything());
    expect(screen.getByText(/lift\.example\.com/)).toBeOnTheScreen();
  });

  it("stays on the server screen when the origin is not a server", async () => {
    onWeb("http://localhost:8081");
    fetch.mockResolvedValue(new Response("<html>metro</html>"));
    await renderApp();
    await at("/server");
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
    expect(screen.getByLabelText("Server address")).toHaveDisplayValue("http://localhost:8081");
    expect(screen.getByRole("button", { name: "Connect" })).toBeOnTheScreen();
  });

  it("signs in to the demo without a network", async () => {
    offline();
    await renderApp();
    await at("/server");
    await press("Try the demo");
    await at("/login");
    await press("Sign in");
    await onToday();
    expect(fetch).not.toHaveBeenCalled();
    expect(JSON.parse(stored.get(KEY)!)).toMatchObject({ serverUrl: MOCK_SERVER, token: /./ });
  });
});

describe("login and register", () => {
  beforeEach(() => storeSession(MOCK_SERVER));

  it("shows the server's message for a wrong password", async () => {
    await renderApp("/login");
    await at("/login");
    await type("Username", "demo");
    await type("Password", "not-the-password");
    await press("Sign in");
    await waitFor(() => expect(screen.getByText("wrong username or password")).toBeOnTheScreen());
    expect(screen).toHavePathname("/login");
  });

  it("links to register and back to the server screen", async () => {
    await renderApp("/login");
    await at("/login");
    await press("Create an account");
    await at("/register");
    await press("Sign in instead");
    await at("/login");
    await press("Change server");
    await at("/server");
    expect(screen.getByLabelText("Server address")).toHaveDisplayValue(MOCK_SERVER);
  });

  it("registers with an invite code", async () => {
    await renderApp("/register");
    await at("/register");
    await type("Display name", "Friend");
    await type("Username", "friend");
    await type("Password", "long-enough-password");
    await type("Invite code", inviteCode);
    await press("Create account");
    await onToday();
    expect(JSON.parse(stored.get(KEY)!).user).toMatchObject({ username: "friend" });
  });

  it("shows the server's hint when the invite is missing", async () => {
    await renderApp("/register");
    await at("/register");
    await type("Username", "friend");
    await type("Password", "long-enough-password");
    await press("Create account");
    await waitFor(() =>
      expect(screen.getByText("a valid invite code is required")).toBeOnTheScreen(),
    );
    expect(screen).toHavePathname("/register");
  });
});
