# Test

## Unit Test

Unit tests are intendend for utilities

## e2e Test

1. Create `.env` file in the project root by executing `cp .env.example .env`

2. Add image registry credentials in `.env` file

3. Run `make e2e.{KUBE_VERSION}`, and you will get a shell (once the shell exited, the e2e environment get destroyed)

  - `KUBE_VERSION` is the kubernetes version but with `.` replaced as `-` (for kind cluster naming)
  - e.g. `make e2e.v1-17` -> test with kubernetes v1.17

4. Set `KUBECONFIG` to `./build/e2e/clusters/{KUBE_VERSION}/kubeconfig.yaml`

  ```bash
  export KUBECONFIG=./build/e2e/clusters/{KUBE_VERSION}/kubeconfig.yaml
  ```
