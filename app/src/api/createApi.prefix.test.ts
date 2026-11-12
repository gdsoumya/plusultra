import { connectBaseUrl } from "./createApi";
import { normalizeServerUrl } from "../data/session";

describe("api prefix", () => {
  it("adds /api/v1 once, even when the stored address already has it", () => {
    expect(connectBaseUrl("http://h:8080")).toBe("http://h:8080/api/v1");
    expect(connectBaseUrl("http://h:8080/api/v1")).toBe("http://h:8080/api/v1");
    expect(normalizeServerUrl("http://h:8080/api/v1/")).toBe("http://h:8080");
  });
});
