export type Point = { x: number; y: number };

// Maps [d0, d1] onto [r0, r1]; a flat domain lands in the middle of the range.
export const linear =
  ([d0, d1]: [number, number], [r0, r1]: [number, number]) =>
  (v: number) =>
    d1 === d0 ? (r0 + r1) / 2 : r0 + ((v - d0) / (d1 - d0)) * (r1 - r0);

// Round tick values covering [min, max] in about `count` steps of 1, 2 or 5 times a power of ten.
export function niceTicks(min: number, max: number, count = 4): number[] {
  if (min === max) [min, max] = [min - 1, max + 1];
  const magnitude = 10 ** Math.floor(Math.log10((max - min) / count));
  const step = [1, 2, 5, 10].map((m) => m * magnitude).find((s) => s >= (max - min) / count)!;
  const decimals = Math.max(0, -Math.floor(Math.log10(magnitude)));
  const lo = Math.floor(min / step) * step;
  const n = Math.ceil((max - lo) / step - 1e-9);
  return Array.from({ length: n + 1 }, (_, i) => Number((lo + i * step).toFixed(decimals)));
}

export const linePath = (points: Point[]) =>
  points.map((p, i) => `${i ? "L" : "M"}${+p.x.toFixed(1)} ${+p.y.toFixed(1)}`).join(" ");
