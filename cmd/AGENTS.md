# Go command standard

This is the repository standard for **all Go code under `cmd`**, including
constructors, auxiliary functions, and tests. It supplements the
[root guidance](../AGENTS.md).

The foundation is ordinary [Go style](https://go.dev/wiki/CodeReviewComments),
[explicit contexts](https://pkg.go.dev/context), error-returning functions,
`io.Reader`/`io.Writer`, and [Cobra](https://pkg.go.dev/github.com/spf13/cobra)
where a command tree is needed. The file layout and naming below are Soda
conventions, not claims that Go requires one universal CLI architecture.

This standard defines how to organize code, not new product behavior. Preserve
existing command names, flags, privilege boundaries, output contracts, and exit
statuses during structural work. Existing files contain inconsistencies and are
not automatically examples of compliance. Establishing this standard does not
itself authorize refactoring every command or changing its interface.

## 1. Ownership and layout

`cmd` owns the executable boundary: production wiring, arguments, input/output,
invocation of domain operations, process lifecycle, and exit status. Domain rules,
native host operations, persistence, and artifact construction belong to their
specific owners under `internal`.

Use the same responsibility map in every executable directory:

| File | Responsibility |
| --- | --- |
| `main.go` | `main` and production `run` entrypoint; native wiring and process lifecycle |
| `command.go` | Cobra root construction or fixed-protocol execution boundary |
| `command_test.go` | Root command or protocol-boundary behavior |
| `<operation>.go` | A substantial operation's constructor, handler, and directly related helpers |
| `<operation>_test.go` | Tests for that operation |

`main` calls `run() int` and passes its result to `os.Exit`. `run` owns production
setup, deferred cleanup, the final diagnostic, and error-to-exit translation.
Returning from `run` must finish cleanup before `main` exits. Only `main` calls
`os.Exit`; do not use `log.Fatal` or process exits in handlers or test helpers.

Create operation files for actual operation boundaries, not to meet a line-count
gate. Keep a small execution boundary together in `command.go`; do not create
empty operation files. Auxiliary functions stay beside the operation they
support. Do not introduce `utils.go`, `common.go`, catch-all helper packages, or a
shared command framework just to make files look alike.

## 2. Command construction

Use one of these two forms, according to the existing executable interface:

- **Command tree:** `newCommand` returns the Cobra root. Each subcommand has a
  named `<operation>Command` constructor, such as `recordCommand`. Constructors
  register flags and handlers; they do not execute operations.
- **Fixed protocol or single operation:** `execute` is the testable execution
  boundary. It accepts the arguments, dependencies, and streams it actually
  needs and returns errors/results to `run`. Do not add Cobra to a JSON helper,
  reporter, or process launcher merely for uniformity.

For Cobra constructors:

- Name the local variable and handler parameter `command`; use `root` when
  distinguishing a parent from its children. Use `flags := command.Flags()`
  when registering local flags.
- Use expanded, one-field-per-line command literals and `RunE`. Use
  `cobra.NoArgs` when no positional arguments are accepted; otherwise declare
  the actual argument contract explicitly.
- Give every command `Use` and `Short`. Set `SilenceUsage` and `SilenceErrors`
  on the root so execution errors have one reporting owner.
- Configure completion explicitly on the root and test the intended inventory.
  Preserve its existing availability during a structural refactor.
- Keep flag-bound options local to their constructor. Mark required flags there
  and test their requiredness. Reuse the domain's options type rather than
  copying it into a CLI-only representation.
- Prefer a short `RunE` closure that delegates to a named operation when needed.
  Do not mix in a generic command factory or a one-off command-state struct just
  to avoid writing the same small registration pattern.

Construction must not query the host, read credentials, contact a service, create
files, acquire locks, or start processes. Help and argument-validation tests must
not need a working host installation.

## 3. Explicit dependencies

Production wiring belongs in `run`. Construct inert adapters there; defer real
work until the requested operation runs, using an injected factory when creation
itself performs work. Pass the dependencies each execution boundary needs
explicitly, using existing domain interfaces or concrete types. Different domains
need not share the same constructor signature.

A handler must not silently replace an injected dependency with `OSRunner`, a
native host, a network client, or a fixed system path. This includes secondary
builders, lock acquisition, current-account lookup, and release verification.
If read-only JSON operations and progress-streaming operations need different
runners or writers, construct and pass them explicitly.

Read process-global inputs such as arguments, environment, working directory,
identity, and standard streams at the production boundary. Testable handlers
receive their inputs rather than rediscovering them through globals. Native
adapters may own real system paths; command tests must be able to substitute the
adapter without touching those paths.

Use a small interface or function for a concrete operation where substitution is
needed. Do not add global function overrides, public test flags, generic service
locators, or dependency bags invented to get under an argument-count limit.
Do not create a parallel process-execution abstraction when an existing owner
already provides the required behavior.

## 4. Context and process lifecycle

For operations that spawn children or perform cancellable work, `run` owns a
signal-aware context for interrupt and termination. Cobra roots execute through
`ExecuteContext`; handlers use `command.Context()`. Fixed execution boundaries
receive `context.Context` as their first argument. Do not store contexts in
option structs or replace the supplied context with `context.Background()`.

Deadlines belong to the operation that needs them. Bound short optional status
lookups so unavailable networking cannot block welcome guidance or native service
fallback. Do not impose the same deadline on a status query and an image build.
Always release cancellation functions and acquired resources. Cleanup that must
outlive cancellation needs an explicit, bounded cleanup context owned by the
operation; do not hide that decision in a generic command utility.

A process launcher is the deliberate exception: successful `exec` replaces the
process and retains native signal, descriptor, and exit behavior. Do not replace
it with a supervising child-process wrapper to fit the ordinary command shape.

## 5. Output, errors, and status

- Cobra handlers use `command.InOrStdin()`, `OutOrStdout()`, and `ErrOrStderr()`;
  fixed execution boundaries receive readers/writers explicitly. Delegated
  output must follow the same routing. No handler-level `fmt.Print*` or access to
  `os.Stdin`, `os.Stdout`, or `os.Stderr`.
- `soda-image oci` emits only its returned absolute archive path on stdout and
  routes progress to stderr. `soda-image publish` uses the selected native spec
  and the archive's own identity, never publisher HEAD. ISO/QCOW2 are independent.
  `soda-acceptance run` consumes actual artifact paths and preserves partial run
  reports; no release-record creation/signing/verification commands remain.
- Use `encoding/json` for JSON protocols: one newline-terminated value per
  response, with the encoder's default escaping unless an external protocol
  requires otherwise. Keep diagnostics and command traces off JSON stdout.
- Human output and native progress may be text. Check output/encoding errors;
  do not report success after failing to write the result.
- Return errors from handlers. Add useful operation context with `%w` when
  wrapping a cause; do not log and return the same error. `run` prints the final
  diagnostic to stderr once, as `<executable>: <message>`, without timestamps.
- Keep exit translation in `run`, not scattered through operation handlers.
  Preserve the existing distinction between usage errors and operation failures;
  adopting this layout is not permission to renumber exit statuses.

Protocol-specific outcomes remain explicit and tested:

- Projects can emit a structured incomplete-removal receipt and then exit
  unsuccessfully. Do not discard that receipt when translating the outcome.
- The privileged workspace helper must retain its transport-success behavior
  for structured removal results; its coordinator interprets completion.
- Tailnet welcome guidance remains non-fatal when status is unavailable.
- Forgejo's Tailnet reporter remains a plain endpoint record, not JSON.
- Updates status/check return native host JSON; check progress stays on stderr.
  Update streams native `bootc upgrade --apply` progress/errors. Root
  authorization and injected runners are mandatory; no browser-selected release,
  version/digest approval, or separate Soda reboot protocol is accepted.

## 6. Auxiliary code and Go style

Use `gofmt`, with standard-library imports separated from other imports. Keep a
blank line between top-level declarations. Use ordinary multi-line declarations
and function bodies rather than dense literals or semicolon-packed statements.
Start error messages in lower case, except for proper names and acronyms, and
omit terminal punctuation.

Name helpers for the operation they perform. Prefer early error returns, local
variables, and the existing domain result/options types. A struct must represent
a cohesive responsibility or actual state, not merely carry unrelated variables
between helper functions. Keep pure formatting functions free of host access.
Do not move branches into assertion helpers or arbitrary wrappers to satisfy
complexity gates.

## 7. Tests and review

Every executable needs command-boundary tests, not only tests of its internal
domain package. Keep those tests beside the boundary or operation they exercise,
using `testing` and the repository's `testify/require` conventions. Use named,
table-driven cases for repeated argument or outcome contracts.

Cover the applicable behaviors:

- construction without side effects; help, command inventory, invalid arguments,
  and required flags;
- dispatch to the supplied dependency with the expected options and context;
- stdout/stderr separation, complete JSON decoding, and writer failures;
- privilege rejection before mutation and no native work after validation fails;
- cancellation and resource release where the command owns them;
- success/failure exit translation and the protocol-specific outcomes above.

Fakes record and check calls; unexpected work must fail the test rather than
silently succeed. Use temporary files, explicit streams, and local test fixtures,
not real accounts, `/run` locks, remote services, or production commands. Use a
subprocess test only when the process boundary itself must be exercised, such as
exit status or `exec`; keep it isolated and disposable.

Test behavior, not where a source string happens to appear. Packaging tests may
check installed paths and artifacts, but CLI tests must not pin a constructor or
flag declaration to a particular source file. A file move must not weaken the
contract being tested.

Before accepting command changes, review each section above and run the focused
tests plus the existing [repository quality gates](../AGENTS.md#quality-gates).
`justfile` and `scripts` remain the source of truth for automated checks. Those
checks do not currently enforce every rule here; passing them alone is not proof
of compliance. Report unavailable Linux or matching-native verification honestly.
