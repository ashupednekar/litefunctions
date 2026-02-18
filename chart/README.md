# litefunctions Helm Chart

Kubernetes-native functions platform with no vendor lock-in and no language lock-in.

## Install

```bash
helm install litefunctions oci://ghcr.io/ashupednekar/charts/litefunctions
```

## Upgrade

```bash
helm upgrade litefunctions oci://ghcr.io/ashupednekar/charts/litefunctions
```

## Uninstall

```bash
helm uninstall litefunctions
```

## Verify

```bash
helm status litefunctions
kubectl get pods -n <namespace>
```

## Docs

- Platform docs: https://ashupednekar.github.io/litefunctions/docs/
- Helm chart guide: https://ashupednekar.github.io/litefunctions/docs/helm-chart/
