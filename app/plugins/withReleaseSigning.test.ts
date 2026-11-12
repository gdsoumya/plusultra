import type { ExportedConfig } from "@expo/config-plugins";

import withReleaseSigning from "./withReleaseSigning";

const template = `android {
    signingConfigs {
        debug {
            storeFile file('debug.keystore')
        }
    }
    buildTypes {
        debug {
            signingConfig signingConfigs.debug
        }
        release {
            // Caution! In production, you need to generate your own keystore file.
            signingConfig signingConfigs.debug
            minifyEnabled enableMinifyInReleaseBuilds
        }
    }
}
`;

async function run(contents: string) {
  const config = withReleaseSigning({ name: "t", slug: "t" }) as ExportedConfig;
  const result = await config.mods!.android!.appBuildGradle!({
    ...config,
    modResults: { contents, language: "groovy", path: "build.gradle" },
    modRawConfig: config,
    modRequest: {
      projectRoot: "",
      platformProjectRoot: "",
      modName: "appBuildGradle",
      platform: "android",
      introspect: true,
    },
  });
  return result.modResults.contents;
}

describe("withReleaseSigning", () => {
  it("signs release with keystore.properties and falls back to debug", async () => {
    const out = await run(template);
    expect(out).toContain("rootProject.file('../keystore.properties')");
    expect(out).toMatch(
      /release \{\n\s+\/\/ Caution![^\n]*\n\s+signingConfig signingConfigs\.release/,
    );
    expect(out).toMatch(/debug \{\n\s+signingConfig signingConfigs\.debug/);
    expect(out.match(/keyAlias 'androiddebugkey'/g)).toHaveLength(1);
  });

  it("is idempotent", async () => {
    const once = await run(template);
    expect(await run(once)).toBe(once);
  });
});
