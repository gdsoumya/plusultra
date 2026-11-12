import { create } from "@bufbuild/protobuf";

import {
  ProgramStructureSchema,
  ScriptErrorSchema,
  type ProgramStructure,
  type ScriptError,
} from "../../gen/plusultra/v1/program_pb";

// shortcut: no Starlark here, so only lines saying "error" fail and the structure comes from
// regexes; upgrade if a screen needs real script errors without a server.
export const scriptErrors = (script: string): ScriptError[] =>
  script.split("\n").flatMap((text, i) => {
    const col = text.search(/\berror\b/);
    const message = 'name "error" is not defined';
    return col < 0 ? [] : [create(ScriptErrorSchema, { line: i + 1, col: col + 1, message })];
  });

export const nameOf = (script: string) => /program\("([^"]+)"/.exec(script)?.[1] ?? "Untitled";

export const loops = (script: string) => !/loop=False/.test(script);

export function structureOf(script: string): ProgramStructure {
  const days = [...script.matchAll(/^day\((\d+),\s*(\d+),\s*"([^"]*)"(.*)$/gm)].map(
    ([, week, day, name, rest]) => ({
      week: Number(week),
      day: Number(day),
      name,
      kind: /kind="(\w+)"/.exec(rest)?.[1] ?? "training",
    }),
  );
  const weeks = Number(/weeks=(\d+)/.exec(script)?.[1] ?? 1);
  return create(ProgramStructureSchema, { weeks, loop: loops(script), days });
}
