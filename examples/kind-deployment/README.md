# Live Deployment health demo

This small demo shows KubeHealth assessing a real Kubernetes Deployment:

1. Create a local kind cluster.
2. Deploy a healthy nginx Deployment and wait for it to become available.
3. Change the Deployment to use an image that does not exist.
4. Wait for Kubernetes to report the failed rollout.
5. Read the Deployment from the Kubernetes API and print its KubeHealth assessment.

The expected result is `Failed + Available + Active`: the requested update
failed, but the old replicas remain available while serving traffic.

The inspector prints all three dimensions as JSON. The key statuses should look
like this:

```json
{
  "reconciliation": { "status": "Failed" },
  "availability": { "status": "Available" },
  "lifecycle": { "status": "Active" }
}
```

The Go program uses client-go to retrieve the object. KubeHealth itself assesses
the object passed to `kubehealth.Assess`; it does not connect to or query the
cluster.

## Requirements

- Docker
- [kind](https://kind.sigs.k8s.io/)
- `kubectl`
- Go 1.26 or newer

## Run

From this directory:

```sh
make demo
```

The demo creates a cluster named `kubehealth-demo` if one does not already
exist. It does not delete or recreate an existing cluster. To assess the
Deployment again after the demo:

```sh
make inspect
```

The demo cluster remains available after the run. Delete it explicitly when you
are done:

```sh
make delete
```

You can also run the steps separately with `make cluster`, `make deploy`,
`make break`, `make wait-failed`, and `make inspect`. The demo uses the
`kind-kubehealth-demo` kubeconfig context and the `default` namespace.

Run the offline check for the expected failed-but-available result with:

```sh
go test ./...
```
