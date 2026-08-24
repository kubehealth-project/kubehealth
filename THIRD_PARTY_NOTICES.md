# Third-party notices

Some KubeHealth ecosystem checks were informed by Argo CD resource health
customizations:

- Argo Rollouts `Rollout`
- cert-manager `Certificate` and `Issuer`
- Kyverno `Policy` and `ClusterPolicy`
- KEDA `ScaledObject`

Argo CD is licensed under the Apache License 2.0:

- <https://github.com/argoproj/argo-cd/blob/master/LICENSE>

KubeHealth's checks use its own three-dimensional assessment contract and do not
include Argo CD's copied health packages or Flux CD's copied kstatus packages.
