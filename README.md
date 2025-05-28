# ConfigMap Reloader Operator

A Kubernetes operator that reloads resources when annotated ConfigMaps change. This is an experimental passion project, and feedback is welcome!

> [!WARNING]
>
> This is an alpha project, so its behavior may evolve.

## How it Works

The ConfigMap Reloader Operator watches for ConfigMaps with the annotation `reloader.k8s.mahyarmirrashed.com/reload: "true"`. When these ConfigMaps change, the operator triggers reloads for associated resources (e.g., Pods).
