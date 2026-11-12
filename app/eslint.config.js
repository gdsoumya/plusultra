const expo = require("eslint-config-expo/flat");

module.exports = [...expo, { ignores: ["dist/*", "android/*", "ios/*", ".expo/*", "src/gen/*"] }];
