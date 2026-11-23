# Deployment

The API is packaged as a container and deployed to Kubernetes with the
manifests in `infra/k8s/`.

1. Tag a release with `scripts/release.sh 1.2.3`.
1. CI builds and pushes `ghcr.io/example/api:1.2.3`.
1. Apply the manifests:

```sh
kubectl apply -f infra/k8s/
```
