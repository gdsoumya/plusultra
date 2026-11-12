import { ProgramKind } from "../../gen/plusultra/v1/program_pb";

export const kindOf = (script: string) =>
  script.includes("meal_program(") ? ProgramKind.MEAL : ProgramKind.WORKOUT;

export const lineCount = (script: string) => script.split("\n").length;

// Character offset of a 1-based line and column; col 0 means the start of the line.
export function offsetOf(script: string, line: number, col: number): number {
  const lines = script.split("\n");
  let offset = 0;
  for (let i = 0; i < line - 1 && i < lines.length; i++) offset += lines[i].length + 1;
  return offset + Math.max(col - 1, 0);
}

export type Cursor = { start: number; end: number };

const EX_LINE = (name: string) => `ex("${name}", sets=[s(3, 5, kg(60))])`;

// Puts a found exercise where the cursor is: a whole ex(...) line at the start of a line, else the
// quoted name. Returns the script and where the cursor lands.
export function insertExercise(script: string, { start, end }: Cursor, name: string) {
  const before = script.slice(0, start);
  const after = script.slice(end);
  const atLineStart = before === "" || before.endsWith("\n");
  const breakAfter = atLineStart && after !== "" && !after.startsWith("\n") ? "\n" : "";
  const text = (atLineStart ? EX_LINE(name) : `"${name}"`) + breakAfter;
  return { script: before + text + after, cursor: start + text.length - breakAfter.length };
}

// Replaces the quoted unknown name on a line with a suggestion.
export function replaceName(script: string, line: number, from: string, to: string) {
  const lines = script.split("\n");
  if (lines[line - 1]) lines[line - 1] = lines[line - 1].replace(`"${from}"`, `"${to}"`);
  return lines.join("\n");
}
