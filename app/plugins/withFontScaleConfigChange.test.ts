import type { AndroidConfig, ExportedConfig } from "@expo/config-plugins";

import withFontScaleConfigChange from "./withFontScaleConfigChange";

const manifest = (configChanges: string): AndroidConfig.Manifest.AndroidManifest => ({
  manifest: {
    $: { "xmlns:android": "http://schemas.android.com/apk/res/android" },
    queries: [],
    application: [
      {
        $: { "android:name": ".MainApplication" },
        activity: [
          { $: { "android:name": ".MainActivity", "android:configChanges": configChanges } },
        ],
      },
    ],
  },
});

async function run(fixture: AndroidConfig.Manifest.AndroidManifest) {
  const config = withFontScaleConfigChange({ name: "t", slug: "t" }) as ExportedConfig;
  const result = await config.mods!.android!.manifest!({
    ...config,
    modResults: fixture,
    modRawConfig: config,
    modRequest: {
      projectRoot: "",
      platformProjectRoot: "",
      modName: "manifest",
      platform: "android",
      introspect: true,
    },
  });
  return result.modResults.manifest.application![0].activity![0].$["android:configChanges"];
}

describe("withFontScaleConfigChange", () => {
  it("appends fontScale and density to the main activity's configChanges", async () => {
    expect(await run(manifest("keyboard|orientation"))).toBe(
      "keyboard|orientation|fontScale|density",
    );
  });

  it("adds only what is missing", async () => {
    expect(await run(manifest("fontScale|keyboard"))).toBe("fontScale|keyboard|density");
    expect(await run(manifest("keyboard|fontScale|density"))).toBe("keyboard|fontScale|density");
  });
});
