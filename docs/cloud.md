# Artifacts, budgets, and reviewed infrastructure

Version 0.5.0 adds local artifact files, a cost check, a cleanup selection rule, and a Terraform file. Nothing in the tests calls AWS.

`cloud.Put` and `cloud.Get` read and write one directory. The artifact name has to be a single path element, so a name cannot leave that directory.

`cloud.EstimateUSD` is `cpuHours * cpuPrice + memoryGiBHours * memoryPrice`. `cloud.Allow` rejects a non-positive budget and any estimate above the budget. The prices are arguments. This file does not hide a price list.

`cloud.Expired` deletes nothing itself. It returns managed resources that have a run id and are older than the TTL. Unmanaged resources and resources with an empty run id are left out.

`deploy/terraform/main.tf` is the reviewed module: a VPC, one subnet, an artifact bucket, an SQS queue, an ECR repository, a worker IAM role limited to that bucket and queue, and a Secrets Manager secret. It does not create an EKS cluster. `deploy/kubernetes/benchmark-job.yaml` is the disposable job shape, with a CPU and memory limit, `activeDeadlineSeconds`, and `ttlSecondsAfterFinished`. Apply the Terraform file only from this repository. Do not apply Terraform that a pull request added.

The Go process still stores artifacts on local disk. Pointing `Put` at S3 is an operator deploy step, not something a manifest can request.
