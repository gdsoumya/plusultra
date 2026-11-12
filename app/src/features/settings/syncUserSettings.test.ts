import { createClients } from "../../api/clients";
import { createMockTransport } from "../../api/mockTransport";
import { LengthUnit, Unit } from "../../gen/plusultra/v1/common_pb";
import { syncUserSettings } from "./syncUserSettings";

it("sends the units, measurement unit, device time zone and name to UpdateMe", async () => {
  const api = createClients(createMockTransport().transport);
  await syncUserSettings(api, { units: "lb", measurementUnit: "in", displayName: "Sam" });
  const { user } = await api.auth.getMe({});
  expect(user).toMatchObject({
    units: Unit.LB,
    measurementUnit: LengthUnit.IN,
    displayName: "Sam",
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
  });
});

it("keeps the server's name when the device has none", async () => {
  const api = createClients(createMockTransport().transport);
  const before = (await api.auth.getMe({})).user?.displayName;
  await syncUserSettings(api, { units: "kg", measurementUnit: "cm", displayName: "" });
  expect((await api.auth.getMe({})).user?.displayName).toBe(before);
});
