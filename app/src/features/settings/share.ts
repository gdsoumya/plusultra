import { Platform, Share } from "react-native";

// Puts text on the clipboard: the clipboard API on the web (a hidden textarea on a plain-http
// page, where the API is missing), the share sheet's Copy elsewhere. Resolves false when nothing
// could copy, so the caller can point at the text to select by hand.
export async function copyText(text: string): Promise<boolean> {
  if (Platform.OS !== "web") {
    await Share.share({ message: text });
    return true;
  }
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return true;
  }
  const area = document.createElement("textarea");
  area.value = text;
  area.setAttribute("readonly", "");
  area.style.position = "fixed";
  area.style.opacity = "0";
  document.body.appendChild(area);
  area.select();
  let copied = false;
  try {
    copied = document.execCommand("copy");
  } finally {
    area.remove();
  }
  return copied;
}

// Hands the export to the user: a download on the web, the share sheet elsewhere.
export async function shareZip(zip: Uint8Array, filename: string) {
  if (Platform.OS === "web") {
    const url = URL.createObjectURL(new Blob([zip.slice()], { type: "application/zip" }));
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    link.click();
    URL.revokeObjectURL(url);
    return;
  }
  // shortcut: without expo-file-system and expo-sharing the bytes cannot be written to a file on
  // the device, so only a note is shared; add those packages to share the zip itself.
  await Share.share({
    title: filename,
    message: `${filename} is ready. Open PlusUltra in a browser and export there to download it.`,
  });
}
