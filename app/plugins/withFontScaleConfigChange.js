const { AndroidConfig, withAndroidManifest } = require("@expo/config-plugins");

// Without these, a system font-scale change recreates MainActivity and a running workout timer is lost.
const CHANGES = ["fontScale", "density"];

/** @type {import("@expo/config-plugins").ConfigPlugin} */
const withFontScaleConfigChange = (config) =>
  withAndroidManifest(config, (c) => {
    const activity = AndroidConfig.Manifest.getMainActivityOrThrow(c.modResults);
    const current = (activity.$["android:configChanges"] ?? "").split("|").filter(Boolean);
    activity.$["android:configChanges"] = [
      ...current,
      ...CHANGES.filter((x) => !current.includes(x)),
    ].join("|");
    return c;
  });

module.exports = withFontScaleConfigChange;
