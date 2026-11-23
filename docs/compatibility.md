# Compatibility report

This report compares tree-commit with pre-commit on 10 real projects. Each project has a `.pre-commit-config.yaml`. For each project, both tools ran all hooks on all files (`run --all-files`), and the results were compared hook by hook.

## Method

1. A GitHub code search found 846 repositories with a `.pre-commit-config.yaml` at the root.
1. A seeded random sample selected 10 of them, of moderate size: at least 300 stars, 2 MB to 80 MB, not archived, and active since June 2025.
1. Each repository was cloned. tree-commit and pre-commit 4.3.0 each ran on a clean checkout.
1. The test environment was Ubuntu 26.04 in WSL, with Python 3.14, Go 1.24, Node.js 26 (LTS) and uv. The hooks are third-party code, so they ran only in this environment.

The harness is in [`tools/compat`](../tools/compat). Use it to run the comparison again.

## Results

The comparison found these problems in tree-commit. All of them are fixed:

| Problem | Effect | Fix |
| --- | --- | --- |
| Debian and Ubuntu do not include `ensurepip` without the `python3-venv` package. | No remote Python hook could run (8 of 10 projects). | tree-commit uses uv to create environments when uv is available. Without uv, it tries `venv` and then `virtualenv`. |
| The file type table was a small part of pre-commit's `identify` table. | Hooks with `types: [cython]` did not run on `.pyx` files. | tree-commit now uses the full tables from `identify`, and its algorithm. |
| prek's `repo: builtin` was not supported. | One project could not run at all. | tree-commit runs these hooks from pre-commit-hooks v6.0.0. |
| Remote hooks in `node` and `golang` were skipped. | prettier, markdownlint, yamlfmt and actionlint did not run (4 projects). | tree-commit installs these hooks with npm and `go install`. |
| Hooks without matching files were not shown. | The output listed fewer hooks than pre-commit. | tree-commit shows these hooks as `(no files to check)Skipped`. |

After the fixes, all 71 hooks that pre-commit ran have the same result in tree-commit:

| Project | Hooks | Same result | tree-commit (cold) | pre-commit (cold) | tree-commit (warm) | pre-commit (warm) |
| --- | --- | --- | --- | --- | --- | --- |
| [nunchux-ai/ComfyUI-nunchaku](https://github.com/nunchux-ai/ComfyUI-nunchaku) | 20 | 20 | 12 s | 49 s | 2 s | 3 s |
| [CookieMonsterTeam/CookieMonster](https://github.com/CookieMonsterTeam/CookieMonster) | 3 | 3 | 6 s | 24 s | 2 s | 17 s |
| [sphinx-gallery/sphinx-gallery](https://github.com/sphinx-gallery/sphinx-gallery) | 8 | 8 | 8 s | 36 s | 1 s | 2 s |
| [fredrikaverpil/creosote](https://github.com/fredrikaverpil/creosote) | 1 | 1 | 3 s | 7 s | 0 s | 1 s |
| [pyapp-kit/magicgui](https://github.com/pyapp-kit/magicgui) | 6 | 6 | 10 s | 26 s | 5 s | 5 s |
| [TorchSim/torch-sim](https://github.com/TorchSim/torch-sim) | pre-commit failed | | 21 s | 0 s | 8 s | 0 s |
| [Quantco/glum](https://github.com/Quantco/glum) | 9 | 9 | 0 s | 0 s | 0 s | 0 s |
| [mozilla-ai/any-agent](https://github.com/mozilla-ai/any-agent) | 15 | 15 | 55 s | 59 s | 51 s | 47 s |
| [FujitsuResearch/OneCompression](https://github.com/FujitsuResearch/OneCompression) | 5 | 5 | 4 s | 8 s | 1 s | 1 s |
| [projecthorus/radiosonde_auto_rx](https://github.com/projecthorus/radiosonde_auto_rx) | 4 | 4 | 2 s | 4 s | 0 s | 1 s |

"Cold" is the first run, with empty caches: both tools download and install the hooks. "Warm" is the second run. The projects ran one at a time.

NOTE: "Same result" includes hooks that fail in both tools, for example because a tool such as `pixi` is not installed. A project passes or fails for the same reasons in both tools.

## Remaining differences

The remaining differences have these causes:

- **prek configs.** pre-commit does not support prek's `repo: builtin`, so it stops with an error on torch-sim. tree-commit runs all 13 hooks of torch-sim.
- **Other languages.** tree-commit skips remote hooks in languages other than `python`, `node` and `golang`, for example `docker_image`, `rust` and `ruby`. None of the 10 projects used them.

On torch-sim, the `ty` hook fails in tree-commit because the project's environment needs a PyTorch wheel for Python 3.15. This is a problem of the project's hook, not of tree-commit.
