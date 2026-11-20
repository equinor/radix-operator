# AGENTS.md

Guidelines for AI coding agents working in this repository.

## Testing

- **Always run `make test` to validate changes.** It runs the whole unit test suite (excluding generated clients and e2e). Unchanged packages are served from the Go test cache, so it is fast enough to run after every change.
- Do not truncate or filter test output in a way that can hide failures (e.g. `| tail -40`). Check the exit code and list every failing test, e.g.:
  ```bash
  make test > "$TMPDIR/unit.log" 2>&1; echo "exit=$?"; grep -E "^(FAIL|panic)|--- FAIL" "$TMPDIR/unit.log"
  ```
- If a failing test is unrelated to your change, verify it also fails on the unmodified base branch, report it explicitly as pre-existing, and do not modify or skip it to make the suite pass.
- When fixing a bug, add a test that fails without the fix and passes with it. Verify both, not just the passing state.
- When a change intentionally alters behaviour, update all tests that assert the old behaviour. Search for every assertion of the changed value instead of relying on a partial test run.

### End-to-end tests

- e2e tests live in [e2e/](e2e/) and run against a Kind cluster (`radix-operator-e2e`, reused between runs). See [e2e/README.md](e2e/README.md).
- Run a single test with:
  ```bash
  cd e2e && go test -v -timeout 40m -run TestName .
  ```
- Run the relevant e2e tests when your change touches CRD schemas, defaulting, validation, webhooks, or pipeline behaviour. If the Kind cluster cannot be created or reached in your environment, do not skip silently: state which e2e tests were not run and why.
- Use e2e tests for behaviour that depends on the Kubernetes API server, such as CRD defaulting, validation rules and webhooks. Unit tests with fake clients do not apply CRD defaults.
- Pipeline-driven tests read `radixconfig.yaml` from [e2e/testdata/gitserver/repos/equinor/](e2e/testdata/gitserver/repos/equinor/)`<app-name>.git/`. Add a new directory there per test application, and reuse `deployToEnvironmentThroughPipeline` to deploy it.

## Generated code

- Run `make generate` after any change to types or kubebuilder markers in [pkg/apis/radix/v1/](pkg/apis/radix/v1/). It regenerates deepcopy, clientsets, CRDs, the JSON schema, mocks and swagger. Never edit generated files by hand.
- Review the generated diff. It should only contain changes you intended.

## RadixApplication types and environment overrides

- Component config is merged with `environmentConfig` when the pipeline builds a RadixDeployment (`pkg/apis/deployment`). Several fields are merged with `mergo.WithOverride`, where any non-empty environment value wins.
- **Do not add `+kubebuilder:default` to fields of types that can be used in `environmentConfig`.** The API server applies the default to the environment-level object, and the merge then cannot tell it apart from an explicit value, which silently overrides the component value (see #2076). Apply defaults in code after merging instead (e.g. `NormalizeConfig()`, or in `GetRadixComponentsForEnv`).
- Changes to how config is merged only take effect for RadixDeployments created afterwards. The operator reconciles the already-merged RadixDeployment.

## Writing code for readability and review

Optimise for the reviewer: a change should be easy to understand from the diff alone.

- **Keep changes focused.** Only change what the task requires. Put unrelated refactors, renames and formatting in separate PRs.
- **Follow existing patterns.** Match the conventions, naming, error handling and test style of the surrounding code before introducing new abstractions.
- **Prefer clear over clever.** Use descriptive names, early returns and small functions with a single purpose. Avoid deep nesting and long parameter lists.
- **Make intent explicit.** Name constants instead of using magic values, and return errors with context (`fmt.Errorf("failed to sync oauth proxy for component %s: %w", name, err)`) instead of logging and continuing.
- **Avoid duplication in tests.** Extract shared setup into helpers (e.g. `deployToEnvironmentThroughPipeline`) and use table-driven tests for variations of the same behaviour.
- Run `make lint` before opening a PR.
