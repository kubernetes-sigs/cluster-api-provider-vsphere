# AGENTS.md

Guidance for AI coding agents working in the Cluster API Provider vSphere (CAPV) repository.

## Communication

- Be concise and direct. No flattery, no preambles.

## Constraints

- Never hand-edit generated files (`zz_generated.*`, CRDs/RBAC/webhook manifests under `config/`, flavors derived from
  `templates/`, `test/e2e/data/**/cluster-template*.yaml` and `clusterclass-quick-start*.yaml`).
  Change the Go types, kubebuilder markers or templates and run `make generate`.
- There are five Go modules: root, `api/`, `test/`, `packaging/`, `hack/tools/`. Run `go build`/`go test` from
  the module that owns the package, and use `make generate-modules` instead of running `go mod tidy` by hand.
- CAPV ships two providers in one manager binary: govmomi (`controllers/`, `pkg/services/govmomi`) and supervisor
  (`controllers/vmware/`, `pkg/services/vmoperator`). Keep them apart, and check whether a fix in one needs a
  counterpart in the other.
- Reach vCenter only through `pkg/session`, never create govmomi clients ad hoc. Production code never imports
  `vm-operator/api`; it uses the hub types (`vmoprvhub`) from `pkg/conversion/api/vmoperator/hub`.
- API types live in `api/` (`govmomi` and `supervisor`). v1beta2 is the hub/storage version. Every field added to v1beta2
  is added identically to v1beta1, so that no custom conversion code is needed.
  Run `make test-conversions` after you change API types.
  Don't add tests for conversion code: the fuzz tests already cover it.
- Use the import aliases enforced by `importas` in `.golangci.yml` (`infrav1`, `vmwarev1`, `clusterv1`, `corev1`,
  `apierrors`, `pkgerrors`, ...).
- New files need the Apache 2.0 header without a copyright year (`make verify-boilerplate`).

## Contributing

- Read [CONTRIBUTING.md](CONTRIBUTING.md#ai-guidance).
- Keep changes focused and reviewable. Don't touch unrelated lines, files or tooling. Mention unrelated issues
  instead of fixing them.
- Add tests that cover new behaviour, not tests that only increase coverage.
- Before finishing, run the tests of the packages you changed and `make lint`.
- When creating or submitting a pull request, disclose whether AI was used and briefly describe how.
- Remind the human author that they are responsible for all submitted changes and refer them to `CONTRIBUTING.md`.
- PR titles start with `:warning:` (breaking), `:sparkles:` (feature), `:bug:` (fix), `:book:` (docs) or
  `:seedling:` (other).
- PR descriptions and commit messages follow the communication guidelines above.
- Don't put `@mentions` or `fixes #...` keywords in commit messages.
- Don't add `Co-authored-by:` lines to commit messages.
- Review comments must be answered by the human author, not by an agent.

## Commands

`make help` lists all targets.

```bash
make lint                # golangci-lint + kube-api-linter
make lint-fix            # same as lint and also fix findings when possible
make test                # unit + envtest tests for the root and api modules; narrow with TEST_ARGS="-run TestX"
make generate            # modules, manifests, deepcopy, conversions, flavors
make verify              # everything CI verifies
```

Run a single test:

```bash
# envtest packages need KUBEBUILDER_ASSETS. `make setup-envtest` prints the correct value.
# vcsim-based tests also need GOVC_BIN_PATH; `make test` sets both.
go test ./controllers/... -run '^TestName$'
# Tests of api/ run from the api module.
(cd api && go test ./... -run '^TestName$')
# Fuzz conversion tests.
go test ./internal/webhooks/conversion/... ./internal/webhooks/vmware/conversion/... ./pkg/conversion/... -run TestFuzzyConversion
```

## Developer docs

CAPV follows the Cluster API developer docs, which live in the Cluster API repository under
`docs/book/src/developer/` and are published at <https://main.cluster-api.sigs.k8s.io/developer/>.
Read them from a sibling checkout (`../cluster-api`, ideally at the version in the `replace` directives of `go.mod`)
or from the book.

Before you change code in one of these areas, read at least the `Rules` section of the matching doc:

| When you change ...                                              | Read                                                                |
|------------------------------------------------------------------|---------------------------------------------------------------------|
| any code in the repository                                       | `providers/overview.md`, `providers/best-practices.md`              |
| API types (`api/`)                                               | `core/API.md`                                                       |
| a controller (`controllers/`, `controllers/vmware/`)             | `core/controllers/overview.md`                                      |
| how CAPV talks to CAPI (InfraCluster, InfraMachine, ... fields)  | `providers/contracts/overview.md`, then the page of the resource    |
| log lines                                                        | `core/logging.md`                                                   |
| unit or integration tests                                        | `core/testing.md`                                                   |
| e2e tests (`test/e2e`, `test/framework`)                         | `core/e2e.md`                                                       |

Those docs describe core Cluster API; these parts don't apply to CAPV:

- `core/overview.md`, and the `<provider>/setup` and `<provider>/reconcilers` layout: CAPV has its own layout
  (`controllers/`, `pkg/manager`).
- Stale cache mitigation (`DeferNextReconcileUntilCacheUpToDate`) in `core/controllers/overview.md`: CAPV controllers
  don't use it.
- `util/test/builder`, generic providers and `CAPI_DISABLE_TEST_ENV` in `core/testing.md`: CAPV tests use the fake
  client, envtest and vcsim.
- The rule "core never imports provider types" and the `util/` / `utils/` compatibility rules.
- The `API Changes` section of the migration doc.

If your change makes a doc in this repository incorrect, update it in the same PR.

## Style

- Name things after the concept, not an implementation detail.
- Comments explain *why*, not *what*; use `// Note:` for rationale and non-obvious coupling. Godoc starts with the
  name of the identifier. Comments describe the current state of the code, not the change that led to it.
- Use contextual logging and follow the rules for log messages and key/value pairs in `core/logging.md`.
- Return errors instead of swallowing them or panicking, also for branches that "should never happen".
- Error messages are specific: `failed to <verb> <Kind> <klog.KObj(obj)>`, using the real Kind. Match the wrapping
  style of the surrounding file (`pkgerrors.Wrapf` or `fmt.Errorf("...: %w")`).
- After copying or renaming code, re-read identifiers, comments, godoc and messages, and check they match the new context.
- Extend existing table-driven tests instead of adding a separate test for the same function; keep one `_test.go`
  file per source file.
- When removing code, remove it completely: tests, import aliases in `.golangci.yml` and doc references included.
