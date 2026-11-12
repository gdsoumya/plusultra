import { readFileSync } from "node:fs";
import { join } from "node:path";

// PNG IHDR: width and height are the big-endian u32s at bytes 16 and 20.
const size = (name: string) => {
  const png = readFileSync(join(__dirname, name));
  return [png.readUInt32BE(16), png.readUInt32BE(20)];
};

describe("brand assets", () => {
  it.each([
    ["icon.png", [1024, 1024]],
    ["android-icon-foreground.png", [1024, 1024]],
    ["android-icon-background.png", [1024, 1024]],
    ["android-icon-monochrome.png", [1024, 1024]],
    ["splash-icon.png", [1024, 800]],
    ["favicon.png", [48, 48]],
    ["notification-icon.png", [96, 96]],
  ])("%s is %j px", (name, px) => {
    expect(size(name)).toEqual(px);
  });
});
