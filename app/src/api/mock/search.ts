// Matches every word of the query in the name or aliases; names starting with the query rank
// first, then `rank` (lower first), then the name.
export function search<T extends { name: string; aliases: string[] }>(
  all: T[],
  query: string,
  limit: number,
  rank: (item: T) => number = () => 0,
): T[] {
  const q = query.trim().toLowerCase();
  const words = q.split(/\s+/).filter(Boolean);
  const starts = (item: T) => (item.name.toLowerCase().startsWith(q) ? 0 : 1);
  return all
    .filter((item) => {
      const text = [item.name, ...item.aliases].join(" ").toLowerCase();
      return words.every((w) => text.includes(w));
    })
    .sort((a, b) => starts(a) - starts(b) || rank(a) - rank(b) || a.name.localeCompare(b.name))
    .slice(0, limit || 20);
}
