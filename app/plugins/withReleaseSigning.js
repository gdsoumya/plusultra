const { withAppBuildGradle } = require("@expo/config-plugins");

// Lives in app/ (not app/android) so `prebuild --clean` can never delete the release keystore.
const RELEASE_SIGNING = `        release {
            def keystoreProperties = new Properties()
            def keystorePropertiesFile = rootProject.file('../keystore.properties')
            if (keystorePropertiesFile.exists()) {
                keystorePropertiesFile.withInputStream { keystoreProperties.load(it) }
                storeFile new File(keystorePropertiesFile.parentFile, keystoreProperties['storeFile'])
                storePassword keystoreProperties['storePassword']
                keyAlias keystoreProperties['keyAlias']
                keyPassword keystoreProperties['keyPassword']
            } else {
                storeFile file('debug.keystore')
                storePassword 'android'
                keyAlias 'androiddebugkey'
                keyPassword 'android'
            }
        }
`;

/** @type {import("@expo/config-plugins").ConfigPlugin} */
const withReleaseSigning = (config) =>
  withAppBuildGradle(config, (c) => {
    const gradle = c.modResults.contents;
    if (!gradle.includes("signingConfigs.release")) {
      c.modResults.contents = gradle
        .replace(
          /(release \{[\s\S]*?)signingConfig signingConfigs\.debug/,
          "$1signingConfig signingConfigs.release",
        )
        .replace(/signingConfigs \{\n/, (m) => m + RELEASE_SIGNING);
    }
    return c;
  });

module.exports = withReleaseSigning;
