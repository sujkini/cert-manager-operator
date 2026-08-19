# CM-852: User-specified proxy `overrideEnv` values overwritten by cluster-wide proxy

## Context

When a user configures custom proxy environment variables via
`spec.controllerConfig.overrideEnv` in the `CertManager` CR (e.g. `HTTP_PROXY`,
`HTTPS_PROXY`, `NO_PROXY`), those values are silently overwritten by the
cluster-wide proxy settings. This defeats the purpose of the `overrideEnv`
feature for proxy customization.

## Root cause

The operand deployment is patched by a slice of ordered `DeploymentHookFunc`
hooks in `newGenericDeploymentController`
(`pkg/controller/deployment/generic_deployment_controller.go`). Hooks are
applied in slice order.

- `withContainerEnvOverrideHook` merges the user's `overrideEnv` into the
  container env.
- `withProxyEnv` merges the cluster proxy vars (read from the operator's own
  environment via `proxy.ReadProxyVarsFromEnv()`) into the container env.

`mergeContainerEnvs(source, override)` gives precedence to its **second**
argument, and both hooks pass the newly-injected values as that second argument.
Because `withProxyEnv` was listed **after** `withContainerEnvOverrideHook`, the
cluster proxy values were applied last and clobbered the user's `overrideEnv`
values for `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY`.

## Fix

Move `withProxyEnv` to run **before** `withContainerEnvOverrideHook` in the
hooks slice. This way the cluster proxy vars are injected first and the user's
`overrideEnv` (applied last) takes precedence — matching the documented API
behavior.

## Acceptance criteria

- User-specified `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY` in
  `spec.controllerConfig.overrideEnv` win over the cluster-wide proxy values.
- When no user override is set, the cluster proxy values are still injected
  (unchanged behavior).

## Changes

1. `pkg/controller/deployment/generic_deployment_controller.go` — reorder
   `withProxyEnv` before `withContainerEnvOverrideHook`; add an explanatory
   comment.
2. `pkg/controller/deployment/deployment_overrides_test.go` — add
   `TestProxyEnvOverridePrecedence` verifying user override precedence, plus a
   `newTestCertManagerInformer` helper.
