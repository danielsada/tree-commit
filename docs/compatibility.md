# Compatibility report

This report compares tree-commit with pre-commit on 10 real projects. Each project has a `.pre-commit-config.yaml`. For each project, both tools ran all hooks on all files (`run --all-files`), and the results were compared hook by hook.

## Method

1. A GitHub code search found 846 repositories with a `.pre-commit-config.yaml` at the root.
1. A seeded random sample selected 10 of them, of moderate size: at least 300 stars, 2 MB to 80 MB, not archived, and active since June 2025.
1. Each repository was cloned. tree-commit and pre-commit 4.3.0 each ran on a clean checkout.
1. The test environment was Ubuntu 26.04 in WSL, with Python 3.14, Go 1.24 and uv. Node.js was not installed.

The harness is in [`tools/compat`](../tools/compat). Use it to run the comparison again.

## Results

The first run found three problems in tree-commit. All three are fixed:

| Problem | Effect | Fix |
| --- | --- | --- |
| Debian and Ubuntu do not include `ensurepip` without the `python3-venv` package. | No remote Python hook could run (8 of 10 projects). | tree-commit uses uv to create environments when uv is available. Without uv, it tries `venv` and then `virtualenv`. |
| The file type table was a small part of pre-commit's `identify` table. | Hooks with `types: [cython]` did not run on `.pyx` files. | tree-commit now uses the full tables from `identify`, and its algorithm. |
| prek's `repo: builtin` was not supported. | One project could not run at all. | tree-commit runs these hooks from pre-commit-hooks v6.0.0. |

After the fixes, the results are:

| Project | Hooks (pre-commit) | Same result | tree-commit | pre-commit |
| --- | --- | --- | --- | --- |
| [nunchux-ai/ComfyUI-nunchaku](https://github.com/nunchux-ai/ComfyUI-nunchaku) | 20 | 17 | 2.3 s | 3.7 s |
| [CookieMonsterTeam/CookieMonster](https://github.com/CookieMonsterTeam/CookieMonster) | 3 | 2 | 0.1 s | 19.9 s |
| [sphinx-gallery/sphinx-gallery](https://github.com/sphinx-gallery/sphinx-gallery) | 8 | 8 | 1.1 s | 1.9 s |
| [fredrikaverpil/creosote](https://github.com/fredrikaverpil/creosote) | 1 | 1 | 0.7 s | 0.8 s |
| [pyapp-kit/magicgui](https://github.com/pyapp-kit/magicgui) | pre-commit failed | | 5.7 s | 34.4 s |
| [TorchSim/torch-sim](https://github.com/TorchSim/torch-sim) | pre-commit failed | | 8.4 s | 0.2 s |
| [Quantco/glum](https://github.com/Quantco/glum) | 9 | 9 | 0.1 s | 0.2 s |
| [mozilla-ai/any-agent](https://github.com/mozilla-ai/any-agent) | 15 | 14 | 55.9 s | 56.8 s |
| [FujitsuResearch/OneCompression](https://github.com/FujitsuResearch/OneCompression) | 5 | 5 | 1.4 s | 1.8 s |
| [projecthorus/radiosonde_auto_rx](https://github.com/projecthorus/radiosonde_auto_rx) | 4 | 4 | 0.4 s | 0.8 s |

The times are for a second run, with warm caches, one project at a time.

NOTE: "Same result" includes hooks that fail in both tools, for example because a tool such as `pixi` is not installed. A project passes or fails for the same reasons in both tools.

## Remaining differences

The remaining differences have these causes:

- **Hooks in other languages.** tree-commit skips remote hooks in `node` and `golang` (prettier, markdownlint, yamlfmt, actionlint). pre-commit installs Node.js and Go environments for them. This is the largest difference. It is also why pre-commit is slower on CookieMonster and magicgui.
- **Hooks without files.** pre-commit shows `(no files to check)Skipped` for hooks that match no file. tree-commit does not show these hooks.
- **pre-commit failures.** pre-commit does not support prek's `repo: builtin` (torch-sim). On magicgui, the actionlint hook needs Go 1.25, and pre-commit stopped with an error.

On torch-sim, the `ty` hook fails in tree-commit because the project's environment needs a PyTorch wheel for Python 3.15. This is a problem of the project's hook, not of tree-commit.
