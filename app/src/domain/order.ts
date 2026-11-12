// Where the item at i sits after moveItem(list, from, to): the moved item lands on to, the ones it
// passed shift by one.
export function movedIndex(i: number, from: number, to: number) {
  if (i === from) return to;
  if (from < i && i <= to) return i - 1;
  if (to <= i && i < from) return i + 1;
  return i;
}

// The list with the item at from moved to to (clamped); the same list when nothing moves.
export function moveItem<T>(list: T[], from: number, to: number): T[] {
  const target = Math.max(0, Math.min(list.length - 1, to));
  if (target === from) return list;
  const out = list.slice();
  const [item] = out.splice(from, 1);
  out.splice(target, 0, item);
  return out;
}
