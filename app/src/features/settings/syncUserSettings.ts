import type { Api } from "../../api/clients";
import type { UserSettings } from "../../data/settings";
import { LengthUnit, Unit } from "../../gen/plusultra/v1/common_pb";

// Mirrors the device's settings to the server so the API and other devices see the same units
// and local dates. The settings stay saved on the device when the call fails.
export const syncUserSettings = (api: Api, settings: UserSettings) =>
  api.auth.updateMe({
    units: settings.units === "lb" ? Unit.LB : Unit.KG,
    measurementUnit: settings.measurementUnit === "in" ? LengthUnit.IN : LengthUnit.CM,
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    ...(settings.displayName && { displayName: settings.displayName }),
  });
