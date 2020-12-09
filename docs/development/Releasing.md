# Releasing

## Creating A New Release

1. Open an issue to request a release with issue template [`release`](../../.github/ISSUE_TEMPLATE/release.md)
1. Go to the issue just opened, finish everything listed in the checklist
1. Tag the `master` branch a new semver with signature, `git tag -s vX.XX.X` (prefix `v` is always required)
1. Push the latest tag: `git push origin vX.XX.X`, and ci pipeline will tend to everything else
1. Once CI pipeline finished successfully, you can find a draft release in the release area, update the release description with content from the release issue you created
   - Notable `Issues/Pull requests` references and descriptions goes to `Features` and `Bug Fixes`
   - Notable `Notice` goes to `Breaking Changes`
