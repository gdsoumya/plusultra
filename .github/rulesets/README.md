# Rulesets

GitHub rulesets for this repository, kept here so they can be re-applied. `main` requires a pull
request with the three CI jobs green, linear history and resolved review threads; `release tags`
limits `v*` tags, which trigger the release workflow, to repository admins. Admins bypass both.

Apply (needs a public repository or GitHub Pro):

```sh
for f in .github/rulesets/*.json; do gh api -X POST repos/gdsoumya/plusultra/rulesets --input "$f"; done
```
