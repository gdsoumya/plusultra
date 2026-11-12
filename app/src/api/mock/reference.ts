import type { ServiceImpl } from "@connectrpc/connect";

import { ReferenceService } from "../../gen/plusultra/v1/reference_pb";
import { gzclpScript } from "../fixtures/gzclp";
import { mealScript } from "../fixtures/meals";

const fence = (script: string) => "```python\n" + script + "```\n";

export const reference = (): ServiceImpl<typeof ReferenceService> => ({
  getProgramLanguageReference: () => ({
    markdown:
      "# Program language\n\nA workout program is a Starlark script: `program(...)`, then `day(...)` " +
      "followed by the day's `ex(...)` calls. Sets come from `s(n, reps, weight)`.\n",
  }),
  getMealLanguageReference: () => ({
    markdown:
      "# Meal program language\n\nA meal program is a Starlark script: `meal_program(...)`, foods from " +
      "`food(...)`, then `day(...)` followed by its `meal(...)` calls.\n",
  }),
  getProgramExamples: () => ({
    markdown: `# Examples\n\n## GZCLP\n\n${fence(gzclpScript)}\n## Meals\n\n${fence(mealScript)}`,
  }),
  getLogFormatReference: () => ({
    markdown:
      "# Log format\n\nOne line per date: `YYYY-MM-DD | status | workout | meals | weight`.\n",
  }),
});
