import { Platform, type TextStyle } from "react-native";

// Dark only. Neutral greys in three surface levels stepping up from the near-black background; the
// accent is the only green (P2-R24: no tint on the neutrals).
export const colors = {
  bg: "#0B0C0F",
  surface: "#15171C",
  surfaceRaised: "#1E2128",
  border: "#2A2E37",
  text: "#F2F3F5",
  textMuted: "#9AA0A6",
  accent: "#B4F23C",
  // The accent at about 12% over surface: a dim wash for the one thing that is "live", the next set
  // and the current tab.
  accentSoft: "#283120",
  onAccent: "#0B0C0F",
  danger: "#FF6B6B",
  success: "#4ADE80",
  warning: "#FBBF24",
  // Neutral facts such as a check-in; neither good nor bad, so not green, amber or red.
  info: "#60A5FA",
  // Set groups in a session: warmups a quiet blue-grey, backoffs and other plan labels a violet that
  // sits apart from both the accent (work sets) and the status colours.
  warmup: "#8A9BB5",
  backoff: "#C4A5FF",
  scrim: "rgba(0, 0, 0, 0.7)",
} as const;

export type Colors = typeof colors;
export type ColorRole = keyof Colors;

type Step = Pick<
  TextStyle,
  "fontSize" | "lineHeight" | "fontWeight" | "letterSpacing" | "fontFamily"
>;

export const monoFont = Platform.select({ ios: "Menlo", default: "monospace" });

// display and title are for numbers and screen titles; mono is the script editor's face.
export const typography = {
  display: { fontSize: 40, lineHeight: 44, fontWeight: "700", letterSpacing: -0.8 },
  title: { fontSize: 28, lineHeight: 32, fontWeight: "700", letterSpacing: -0.4 },
  heading: { fontSize: 20, lineHeight: 26, fontWeight: "600", letterSpacing: -0.2 },
  body: { fontSize: 16, lineHeight: 22, fontWeight: "400", letterSpacing: 0 },
  label: { fontSize: 14, lineHeight: 18, fontWeight: "600", letterSpacing: 0.1 },
  caption: { fontSize: 13, lineHeight: 18, fontWeight: "400", letterSpacing: 0 },
  mono: { fontSize: 14, lineHeight: 20, fontWeight: "400", letterSpacing: 0, fontFamily: monoFont },
} as const satisfies Record<string, Step>;

export type TextVariant = keyof typeof typography;

export const tabular: TextStyle = { fontVariant: ["tabular-nums"] };

// Keys are multiples of 4: space[4] === 16.
export const space = { 1: 4, 2: 8, 3: 12, 4: 16, 5: 20, 6: 24, 8: 32, 10: 40 } as const;

export const radius = { control: 10, card: 16, pill: 999 } as const;

// fling: seconds of release velocity projected ahead when picking a snap point.
export const motion = {
  fast: 150,
  normal: 250,
  easing: [0.2, 0, 0, 1],
  fling: 0.15,
  longPress: 300,
} as const;

export const minTouch = 44;

// Control heights: chips and icon tiles, buttons and fields, list rows.
export const control = { sm: 36, md: 48, lg: 56 } as const;

export const iconSize = { sm: 16, md: 20, lg: 24, xl: 40 } as const;

// focus: the ring around a focused field or the active set.
export const stroke = { control: 1, focus: 2, progress: 6 } as const;

export const letterSpacing = { label: 0.8 } as const;

export const pressedOpacity = 0.85;
export const disabledOpacity = 0.4;
// A completed set row: readable, but clearly behind the pending ones.
export const doneOpacity = 0.6;
export const pressedScale = 0.98;

export const handle = { width: 40, height: 4 } as const;

// Content column on wide windows: lists and forms, and grids or charts that can use more.
export const maxWidth = { content: 720, wide: 960 } as const;

// A window at least this wide shows the tabs as a top bar over the content column.
export const wideBreakpoint = 900;
