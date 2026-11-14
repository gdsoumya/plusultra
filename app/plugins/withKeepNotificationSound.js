const { withDangerousMod } = require("@expo/config-plugins");
const fs = require("fs");
const path = require("path");

// expo-notifications copies the timer sound to res/raw/end.wav, but nothing references R.raw.end
// in code, so release resource shrinking drops it and the channels fall back to the system sound.
const KEEP =
  '<?xml version="1.0" encoding="utf-8"?>\n<resources xmlns:tools="http://schemas.android.com/tools" tools:keep="@raw/end" />\n';

/** @type {import("@expo/config-plugins").ConfigPlugin} */
const withKeepNotificationSound = (config) =>
  withDangerousMod(config, [
    "android",
    (c) => {
      const raw = path.join(c.modRequest.platformProjectRoot, "app", "src", "main", "res", "raw");
      fs.mkdirSync(raw, { recursive: true });
      fs.writeFileSync(path.join(raw, "keep.xml"), KEEP);
      return c;
    },
  ]);

module.exports = withKeepNotificationSound;
