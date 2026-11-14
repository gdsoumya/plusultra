import type { ExportedConfig } from "@expo/config-plugins";
import * as fs from "fs";
import * as os from "os";
import * as path from "path";

import withKeepNotificationSound from "./withKeepNotificationSound";

describe("withKeepNotificationSound", () => {
  it("keeps the raw end sound through release resource shrinking", async () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), "keep-"));
    const config = withKeepNotificationSound({ name: "t", slug: "t" }) as ExportedConfig;
    await config.mods!.android!.dangerous!({
      ...config,
      modResults: {},
      modRawConfig: config,
      modRequest: {
        projectRoot: root,
        platformProjectRoot: root,
        modName: "dangerous",
        platform: "android",
        introspect: false,
      },
    });
    const keep = fs.readFileSync(path.join(root, "app/src/main/res/raw/keep.xml"), "utf8");
    expect(keep).toContain('tools:keep="@raw/end"');
  });
});
