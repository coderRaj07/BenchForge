# Capacity and canary gates

Version 0.6.0 estimates capacity from measured samples and decides whether a canary step may advance. Neither function talks to Kubernetes.

## Capacity

`capacity.Analyze` walks samples in the order given. A sample counts only when the generator was not saturated and p95 and error rate stay inside the SLO.

The sustainable rate is the highest achieved rate among those samples. Little's Law is `achieved requests per second × mean latency in seconds`. CPU cores are `achieved rate × CPU seconds per request / target utilization × (1 + headroom)`. Memory is the largest measured working set times `(1 + headroom)`. It is not computed from the request rate.

Confidence stays `low` below three valid samples, `medium` at three or four, and `high` only when there are at least five valid samples and a later sample shows the first limit. A sweep made only of saturated generator samples is not valid capacity.

## Canary

`canary.Decide` returns `pause`, `rollback`, or `promote`.

- Fewer samples than the minimum pauses.
- An error rate above the cap, or a p95 increase above the allowed percent, rolls back.
- A passing step advances to the next weight in the list. The last step promotes in place.

`deploy/argo-rollouts/orders-rollout.yaml` is the rollout shape with weights 10, 25, 50, and 100. The decision function is what the analysis job should call. This repository does not install Argo Rollouts, and the benchmark must not point at a shared production database.
