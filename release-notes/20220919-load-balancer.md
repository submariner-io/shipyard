<!-- markdownlint-disable MD041 -->
Support was added in the Shipyard project to easily deploy Submariner with a LoadBalancer type Service in front.
To use, simply specify the target (e.g. `deploy`) with `USING=load-balancer` or `LOAD_BALANCER=true`.
For kind-based deployments, [MetalLB](https://metallb.universe.tf/) is deployed to provide the capability.
The MetalLB version can be specified using `METALLB_VERSION=x.y.z`.
