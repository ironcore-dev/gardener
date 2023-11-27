# Gardener Operator

The `gardener-operator` is responsible for the garden cluster environment.
Without this component, users must deploy ETCD, the Gardener control plane, etc., manually and with separate mechanisms (not maintained in this repository).
This is quite unfortunate since this requires separate tooling, processes, etc.
A lot of production- and enterprise-grade features were built into Gardener for managing the seed and shoot clusters, so it makes sense to re-use them as much as possible also for the garden cluster.

## Deployment

There is a [Helm chart](../../charts/gardener/operator) which can be used to deploy the `gardener-operator`.
Once deployed and ready, you can create a `Garden` resource.
Note that there can only be one `Garden` resource per system at a time.

> ℹ️ Similar to seed clusters, garden runtime clusters require a [VPA](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler), see [this section](#vertical-pod-autoscaler).
> By default, `gardener-operator` deploys the VPA components.
> However, when there already is a VPA available, then set `.spec.runtimeCluster.settings.verticalPodAutoscaler.enabled=false` in the `Garden` resource.

## `Garden` Resources

Please find an exemplary `Garden` resource [here](../../example/operator/20-garden.yaml).

### Settings For Runtime Cluster

The `Garden` resource offers a few settings that are used to control the behaviour of `gardener-operator` in the runtime cluster.
This section provides an overview over the available settings in `.spec.runtimeCluster.settings`:

#### Load Balancer Services

`gardener-operator` deploys Istio and relevant resources to the runtime cluster in order to expose the `virtual-garden-kube-apiserver` service (similar to how the `kube-apiservers` of shoot clusters are exposed).
In most cases, the `cloud-controller-manager` (responsible for managing these load balancers on the respective underlying infrastructure) supports certain customization and settings via annotations.
[This document](https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer) provides a good overview and many examples.

By setting the `.spec.settings.loadBalancerServices.annotations` field the Gardener administrator can specify a list of annotations which will be injected into the `Service`s of type `LoadBalancer`.

#### Vertical Pod Autoscaler

`gardener-operator` heavily relies on the Kubernetes [`vertical-pod-autoscaler` component](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler).
By default, the `Garden` controller deploys the VPA components into the `garden` namespace of the respective runtime cluster.
In case you want to manage the VPA deployment on your own or have a custom one, then you might want to disable the automatic deployment of `gardener-operator`.
Otherwise, you might end up with two VPAs which will cause erratic behaviour.
By setting the `.spec.settings.verticalPodAutoscaler.enabled=false` you can disable the automatic deployment.

⚠️ In any case, there must be a VPA available for your runtime cluster.
Using a runtime cluster without VPA is not supported.

#### Topology-Aware Traffic Routing

Refer to the [Topology-Aware Traffic Routing documentation](../operations/topology_aware_routing.md) as this document contains the documentation for the topology-aware routing setting for the garden runtime cluster.

#### ETCD Encryption Config

The `spec.virtualCluster.kubernetes.kubeAPIServer.encryptionConfig` field in the Garden API allows users to customize encryption configurations for the API server. It provides options to specify additional resources for encryption.

- The resources field can be used to specify resources that should be encrypted in addition to secrets. The following resources are always encrypted:
  - secrets
  - controllerdeployments.core.gardener.cloud
  - controllerregistrations.core.gardener.cloud
  - internalsecrets.core.gardener.cloud
  - shootstates.core.gardener.cloud
- Adding a resource to this list will cause empty patch requests for all added resources to encrypt them in the etcd. See [Encrypting Confidential Data at Rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data) for more details.
- Removing a resource from this list will cause empty patch requests for all removed resources to decrypt and rewrite the resource as plain text. See [Decrypt Confidential Data that is Already Encrypted at Rest](https://kubernetes.io/docs/tasks/administer-cluster/decrypt-data/) for more details.

> ℹ️ Note that configuring encryption for a custom resource is only supported for  versions >= 1.26.

## Controllers

As of today, the `gardener-operator` only has two controllers which are now described in more detail.

### [`Garden` Controller](../../pkg/operator/controller/garden)

The Garden controller in the operator reconciles Garden objects with the help of the following reconcilers.

#### [`Main` Reconciler](../../pkg/operator/controller/garden/garden)

The reconciler first generates a general CA certificate which is valid for ~`30d` and auto-rotated when 80% of its lifetime is reached.
Afterwards, it brings up the so-called "garden system components".
The [`gardener-resource-manager`](./resource-manager.md) is deployed first since its `ManagedResource` controller will be used to bring up the remainders.

Other system components are:

- runtime garden system resources ([`PriorityClass`es](../development/priority-classes.md) for the workload resources)
- virtual garden system resources (RBAC rules)
- Vertical Pod Autoscaler (if enabled via `.spec.runtimeCluster.settings.verticalPodAutoscaler.enabled=true` in the `Garden`)
- HVPA Controller (when `HVPA` feature gate is enabled)
- ETCD Druid
- Istio

As soon as all system components are up, the reconciler deploys the virtual garden cluster.
It comprises out of two ETCDs (one "main" etcd, one "events" etcd) which are managed by ETCD Druid via `druid.gardener.cloud/v1alpha1.Etcd` custom resources.
The whole management works similar to how it works for `Shoot`s, so you can take a look at [this document](etcd.md) for more information in general.

The virtual garden control plane components are:

- `virtual-garden-etcd-main`
- `virtual-garden-etcd-events`
- `virtual-garden-kube-apiserver`
- `virtual-garden-kube-controller-manager`
- `virtual-garden-gardener-resource-manager`

If the `.spec.virtualCluster.controlPlane.highAvailability={}` is set then these components will be deployed in a "highly available" mode.
For ETCD, this means that there will be 3 replicas each.
This works similar like for `Shoot`s (see [this document](../usage/shoot_high_availability.md)) except for the fact that there is no failure tolerance type configurability.
The `gardener-resource-manager`'s [HighAvailabilityConfig webhook](resource-manager.md#high-availability-config) makes sure that all pods with multiple replicas are spread on nodes, and if there are at least two zones in `.spec.runtimeCluster.provider.zones` then they also get spread across availability zones.

> If once set, removing `.spec.virtualCluster.controlPlane.highAvailability` again is not supported.

The `virtual-garden-kube-apiserver` `Deployment` is exposed via a `Service` of type `LoadBalancer` with the same name.
In the future, we will switch to exposing it via Istio, similar to how the `kube-apiservers` of shoot clusters are exposed.

Similar to the `Shoot` API, the version of the virtual garden cluster is controlled via `.spec.virtualCluster.kubernetes.version`.
Likewise, specific configuration for the control plane components can be provided in the same section, e.g. via `.spec.virtualCluster.kubernetes.kubeAPIServer` for the `kube-apiserver` or `.spec.virtualCluster.kubernetes.kubeControllerManager` for the `kube-controller-manager`.

The `kube-controller-manager` only runs a very few controllers that are necessary in the scenario of the virtual garden.
Most prominently, **the `serviceaccount-token` controller is unconditionally disabled**.
Hence, the usage of static `ServiceAccount` secrets is not supported generally.
Instead, the [`TokenRequest` API](https://kubernetes.io/docs/reference/kubernetes-api/authentication-resources/token-request-v1/) should be used.
Third-party components that need to communicate with the virtual cluster can leverage the [`gardener-resource-manager`'s `TokenRequestor` controller](resource-manager.md#tokenrequestor-controller) and the generic kubeconfig, just like it works for `Shoot`s.
Please note, that this functionality is restricted to the `garden` namespace. The current `Secret` name of the generic kubeconfig can be found in the annotations (key: `generic-token-kubeconfig.secret.gardener.cloud/name`) of the `Garden` resource.

For the virtual cluster, it is essential to provide a DNS domain via `.spec.virtualCluster.dns.domain`.
**The respective DNS record is not managed by `gardener-operator` and should be manually created and pointed to the load balancer IP of the `virtual-garden-kube-apiserver` `Service`.**
The DNS domain is used for the `server` in the kubeconfig, and for configuring the `--external-hostname` flag of the API server.

Apart from the control plane components of the virtual cluster, the reconcile also deploys the control plane components of Gardener.
`gardener-apiserver` reuses the same ETCDs like the `virtual-garden-kube-apiserver`, so all data related to the "the garden cluster" is stored together and "isolated" from ETCD data related to the runtime cluster.
This drastically simplifies backup and restore capabilities (e.g., moving the virtual garden cluster from one runtime cluster to another).

The Gardener control plane components are:

- `gardener-apiserver`
- `gardener-admission-controller`
- `gardener-controller-manager`
- `gardener-scheduler`

The reconciler also manages a few observability-related components (more planned as part of [GEP-19](../proposals/19-migrating-observability-stack-to-operators.md)):

- `fluent-operator`
- `fluent-bit`
- `gardener-metrics-exporter`
- `kube-state-metrics`
- `plutono`
- `vali`

It is also mandatory to provide an IPv4 CIDR for the service network of the virtual cluster via `.spec.virtualCluster.networking.services`.
This range is used by the API server to compute the cluster IPs of `Service`s.

The controller maintains the `.status.lastOperation` which indicates the status of an operation.

#### [`Care` Reconciler](../../pkg/operator/controller/garden/care)

This reconciler performs four "care" actions related to `Garden`s.

It maintains the following conditions:

- `RuntimeComponentsHealthy`: The conditions of the `ManagedResource`s applied to the runtime cluster are checked (e.g., `ResourcesApplied`).
- `VirtualComponentsHealthy`: The virtual components are considered healthy when the respective `Deployment`s (for example `virtual-garden-kube-apiserver`,`virtual-garden-kube-controller-manager`), and `Etcd`s (for example `virtual-garden-etcd-main`) exist and are healthy. Additionally, the conditions of the `ManagedResource`s applied to the virtual cluster are checked (e.g., `ResourcesApplied`).
- `VirtualGardenAPIServerAvailable`: The `/healthz` endpoint of the garden's `virtual-garden-kube-apiserver` is called and considered healthy when it responds with `200 OK`.
- `ObservabilityComponentsHealthy`: This condition is considered healthy when the respective `Deployment`s (for example `plutono`) and `StatefulSet`s (for example `prometheus`, `vali`) exist and are healthy.

If all checks for a certain condition are succeeded, then its `status` will be set to `True`.
Otherwise, it will be set to `False` or `Progressing`.

If at least one check fails and there is threshold configuration for the conditions (in `.controllers.gardenCare.conditionThresholds`), then the status will be set:

- to `Progressing` if it was `True` before.
- to `Progressing` if it was `Progressing` before and the `lastUpdateTime` of the condition does not exceed the configured threshold duration yet.
- to `False` if it was `Progressing` before and the `lastUpdateTime` of the condition exceeds the configured threshold duration.

The condition thresholds can be used to prevent reporting issues too early just because there is a rollout or a short disruption.
Only if the unhealthiness persists for at least the configured threshold duration, then the issues will be reported (by setting the status to `False`).

#### [`Reference` Reconciler](../../pkg/operator/controller/garden/reference)

`Garden` objects may specify references to other objects in the Garden cluster which are required for certain features.
For example, operators can configure a secret for ETCD backup via `.spec.virtualCluster.etcd.main.backup.secretRef.name` or an audit policy `ConfigMap` via `.spec.virtualCluster.kubernetes.kubeAPIServer.auditConfig.auditPolicy.configMapRef.name`.
Such objects need a special protection against deletion requests as long as they are still being referenced by the `Garden`.

Therefore, this reconciler checks `Garden`s for referenced objects and adds the finalizer `gardener.cloud/reference-protection` to their `.metadata.finalizers` list.
The reconciled `Garden` also gets this finalizer to enable a proper garbage collection in case the `gardener-operator` is offline at the moment of an incoming deletion request.
When an object is not actively referenced anymore because the `Garden` specification has changed is in deletion, the controller will remove the added finalizer again so that the object can safely be deleted or garbage collected.

This reconciler inspects the following references:

- ETCD backup `Secret`s (`.spec.virtualCluster.etcd.main.backup.secretRef`)
- Admission plugin kubeconfig `Secret`s (`.spec.virtualCluster.kubernetes.kubeAPIServer.admissionPlugins[].kubeconfigSecretName` and `.spec.virtualCluster.gardener.gardenerAPIServer.admissionPlugins[].kubeconfigSecretName`)
- Authentication webhook kubeconfig `Secret`s (`.spec.virtualCluster.kubernetes.kubeAPIServer.authentication.webhook.kubeconfigSecretName`)
- Audit webhook kubeconfig `Secret`s (`.spec.virtualCluster.kubernetes.kubeAPIServer.auditWebhook.kubeconfigSecretName` and `.spec.virtualCluster.gardener.gardenerAPIServer.auditWebhook.kubeconfigSecretName`)
- SNI `Secret`s (`.spec.virtualCluster.kubernetes.kubeAPIServer.sni.secretName`)
- Audit policy `ConfigMap`s (`.spec.virtualCluster.kubernetes.kubeAPIServer.auditConfig.auditPolicy.configMapRef.name` and `.spec.virtualCluster.gardener.gardenerAPIServer.auditConfig.auditPolicy.configMapRef.name`)

Further checks might be added in the future.

### [`NetworkPolicy` Controller Registrar](../../pkg/controller/networkpolicy)

This controller registers the same `NetworkPolicy` controller which is also used in `gardenlet`, please read it up [here](gardenlet.md#networkpolicy-controllerpkggardenletcontrollernetworkpolicy) for more details.

The registration happens as soon as the `Garden` resource is created.
It contains the networking information of the garden runtime cluster which is required configuration for the `NetworkPolicy` controller.

## Webhooks

As of today, the `gardener-operator` only has one webhook handler which is now described in more detail.

### Validation

This webhook handler validates `CREATE`/`UPDATE`/`DELETE` operations on `Garden` resources.
Simple validation is performed via [standard CRD validation](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation).
However, more advanced validation is hard to express via these means and is performed by this webhook handler.

Furthermore, for deletion requests, it is validated that the `Garden` is annotated with a deletion confirmation annotation, namely `confirmation.gardener.cloud/deletion=true`.
Only if this annotation is present it allows the `DELETE` operation to pass.
This prevents users from accidental/undesired deletions.

Another validation is to check that there is only one `Garden` resource at a time.
It prevents creating a second `Garden` when there is already one in the system.

### Defaulting

This webhook handler mutates the `Garden` resource on `CREATE`/`UPDATE`/`DELETE` operations.
Simple defaulting is performed via [standard CRD defaulting](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#defaulting).
However, more advanced defaulting is hard to express via these means and is performed by this webhook handler.

## Using Garden Runtime Cluster As Seed Cluster

In production scenarios, you probably wouldn't use the Kubernetes cluster running `gardener-operator` and the Gardener control plane (called "runtime cluster") as seed cluster at the same time.
However, such setup is technically possible and might simplify certain situations (e.g., development, evaluation, ...).

If the runtime cluster is a seed cluster at the same time, [`gardenlet`'s `Seed` controller](./gardenlet.md#seed-controller) will not manage the components which were already deployed (and reconciled) by `gardener-operator`.
As of today, this applies to:

- `gardener-resource-manager`
- `vpa-{admission-controller,recommender,updater}`
- `hvpa-controller` (when `HVPA` feature gate is enabled)
- `etcd-druid`
- `istio` control-plane
- `nginx-ingress-controller`

Those components are so-called "seed system components".
In addition, there are a few observability components:

- `fluent-operator`
- `fluent-bit`
- `vali`
- `plutono`
- `kube-state-metrics`

As all of these components are managed by `gardener-operator` in this scenario, the `gardenlet` just skips them.

> ℹ️ There is no need to configure anything - the `gardenlet` will automatically detect when its seed cluster is the garden runtime cluster at the same time.

⚠️ Note that such setup requires that you upgrade the versions of `gardener-operator` and `gardenlet` in lock-step.
Otherwise, you might experience unexpected behaviour or issues with your seed or shoot clusters.

## Credentials Rotation

The credentials rotation works in the same way as it does for `Shoot` resources, i.e. there are `gardener.cloud/operation` annotation values for starting or completing the rotation procedures.

For certificate authorities, `gardener-operator` generates one which is automatically rotated roughly each month (`ca-garden-runtime`) and several CAs which are **NOT** automatically rotated but only on demand.

**🚨 Hence, it is the responsibility of the (human) operator to regularly perform the credentials rotation.**

Please refer to [this document](../usage/shoot_credentials_rotation.md#gardener-provided-credentials) for more details. As of today, `gardener-operator` only creates the following types of credentials (i.e., some sections of the document don't apply for `Garden`s and can be ignored):

- certificate authorities (and related server and client certificates)
- ETCD encryption key
- observability password For Plutono
- `ServiceAccount` token signing key

⚠️ Rotation of static `ServiceAccount` secrets is not supported since the `kube-controller-manager` does not enable the `serviceaccount-token` controller.

When the `ServiceAccount` token signing key rotation is in `Preparing` phase, then `gardener-operator` annotates all `Seed`s with `gardener.cloud/operation=renew-garden-access-secrets`.
This causes `gardenlet` to populate new `ServiceAccount` tokens for the garden cluster to all extensions, which are now signed with the new signing key.
Read more about it [here](../extensions/garden-api-access.md#renewing-all-garden-access-secrets).

Similarly, when the CA certificate rotation is in `Preparing` phase, then `gardener-operator` annotates all `Seed`s with `gardener.cloud/operation=renew-kubeconfig`.
This causes `gardenlet` to request a new client certificate for its garden cluster kubeconfig, which is now signed with the new client CA, and which also contains the new CA bundle for the server certificate verification.
Read more about it [here](gardenlet.md#rotate-certificates-using-bootstrap-kubeconfig).

## Migrating an Existing Gardener Landscape to `gardener-operator`

Since `gardener-operator` was only developed in 2023, six years after the Gardener project initiation, most users probably already have an existing Gardener landscape.
The most prominent installation procedure is [garden-setup](https://github.com/gardener/garden-setup), however experience shows that most community members have developed their own tooling for managing the garden cluster and the Gardener control plane components.

> Consequently, providing a general migration guide is not possible since the detailed steps vary heavily based on how the components were set up previously.
> As a result, this section can only highlight the most important caveats and things to know, while the concrete migration steps must be figured out individually based on the existing installation.
>
> Please test your migration procedure thoroughly.
Note that in some cases it can be easier to set up a fresh landscape with `gardener-operator`, restore the ETCD data, switch the DNS records, and issue new credentials for all clients.

Please make sure that you configure all your desired fields in the [`Garden` resource](#garden-resources).

### ETCD

`gardener-operator` leverages `etcd-druid` for managing the `virtual-garden-etcd-main` and `virtual-garden-etcd-events`, similar to how shoot cluster control planes are handled.
The `PersistentVolumeClaim` names differ slightly - for `virtual-garden-etcd-events` it's `virtual-garden-etcd-events-virtual-garden-etcd-events-0`, while for `virtual-garden-etcd-main` it's `main-virtual-garden-etcd-virtual-garden-etcd-main-0`.
The easiest approach for the migration is to make your existing ETCD volumes follow the same naming scheme.
Alternatively, backup your data, let `gardener-operator` take over ETCD, and then [restore](https://github.com/gardener/etcd-backup-restore/blob/master/docs/operations/manual_restoration.md) your data to the new volume.

The backup bucket must be created separately, and its name as well as the respective credentials must be provided via the `Garden` resource in `.spec.virtualCluster.etcd.main.backup`.

### `virtual-garden-kube-apiserver` Deployment

`gardener-operator` deploys a `virtual-garden-kube-apiserver` into the runtime cluster.
This `virtual-garden-kube-apiserver` spans a new cluster, called the virtual cluster.
There are a few certificates and other credentials that should not change during the migration.
You have to prepare the environment accordingly by leveraging the [secret's manager capabilities](../development/secrets_management.md#migrating-existing-secrets-to-secretsmanager).

- The existing Cluster CA `Secret` should be labeled with `secrets-manager-use-data-for-name=ca`.
- The existing Client CA `Secret` should be labeled with `secrets-manager-use-data-for-name=ca-client`.
- The existing Front Proxy CA `Secret` should be labeled with `secrets-manager-use-data-for-name=ca-front-proxy`.
- The existing Service Account Signing Key `Secret` should be labeled with `secrets-manager-use-data-for-name=service-account-key`.
- The existing ETCD Encryption Key `Secret` should be labeled with `secrets-manager-use-data-for-name=kube-apiserver-etcd-encryption-key`.

### `virtual-garden-kube-apiserver` Exposure

The `virtual-garden-kube-apiserver` is exposed via a dedicated `istio-ingressgateway` deployed to namespace `virtual-garden-istio-ingress`.
The `virtual-garden-kube-apiserver` `Service` in the `garden` namespace is only of type `ClusterIP`.
Consequently, DNS records for this API server must target the load balancer IP of the `istio-ingressgateway`.

### Virtual Garden Kubeconfig

`gardener-operator` does not generate any static token or likewise for access to the virtual cluster.
Ideally, human users access it via OIDC only.
Alternatively, you can create an auto-rotated token that you can use for automation like CI/CD pipelines:

```yaml
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: shoot-access-virtual-garden
  namespace: garden
  labels:
    resources.gardener.cloud/purpose: token-requestor
  annotations:
    serviceaccount.resources.gardener.cloud/name: virtual-garden-user
    serviceaccount.resources.gardener.cloud/namespace: kube-system
    serviceaccount.resources.gardener.cloud/token-expiration-duration: 3h
---
apiVersion: v1
kind: Secret
metadata:
  name: managedresource-virtual-garden-access
  namespace: garden
type: Opaque
stringData:
  clusterrolebinding____gardener.cloud.virtual-garden-access.yaml: |
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRoleBinding
    metadata:
      name: gardener.cloud.sap:virtual-garden
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: cluster-admin
    subjects:
    - kind: ServiceAccount
      name: virtual-garden-user
      namespace: kube-system
---
apiVersion: resources.gardener.cloud/v1alpha1
kind: ManagedResource
metadata:
  name: virtual-garden-access
  namespace: garden
spec:
  secretRefs:
  - name: managedresource-virtual-garden-access
```

The `shoot-access-virtual-garden` `Secret` will get a `.data.token` field which can be used to authenticate against the virtual garden cluster.
See also [this document](resource-manager.md#tokenrequestor-controller) for more information about the `TokenRequestor`.

### `gardener-apiserver`

Similar to the [`virtual-garden-kube-apiserver`](#virtual-garden-kube-apiserver-deployment), the `gardener-apiserver` also uses a few certificates and other credentials that should not change during the migration.
Again, you have to prepare the environment accordingly by leveraging the [secret's manager capabilities](../development/secrets_management.md#migrating-existing-secrets-to-secretsmanager).

- The existing ETCD Encryption Key `Secret` should be labeled with `secrets-manager-use-data-for-name=gardener-apiserver-etcd-encryption-key`.

Also note that `gardener-operator` manages the `Service` and `Endpoints` resources for the `gardener-apiserver` in the virtual cluster within the `kube-system` namespace (`garden-setup` uses the `garden` namespace).

## Local Development

The easiest setup is using a local [KinD](https://kind.sigs.k8s.io/) cluster and the [Skaffold](https://skaffold.dev/) based approach to deploy and develop the `gardener-operator`.

### Setting Up the KinD Cluster (runtime cluster)

```shell
make kind-operator-up
```

This command sets up a new KinD cluster named `gardener-local` and stores the kubeconfig in the `./example/gardener-local/kind/operator/kubeconfig` file.

> It might be helpful to copy this file to `$HOME/.kube/config`, since you will need to target this KinD cluster multiple times.
Alternatively, make sure to set your `KUBECONFIG` environment variable to `./example/gardener-local/kind/operator/kubeconfig` for all future steps via `export KUBECONFIG=$PWD/example/gardener-local/kind/operator/kubeconfig`.

All the following steps assume that you are using this kubeconfig.

### Setting Up Gardener Operator

```shell
make operator-up
```

This will first build the base images (which might take a bit if you do it for the first time).
Afterwards, the Gardener Operator resources will be deployed into the cluster.

### Developing Gardener Operator (Optional)

```shell
make operator-dev
```

This is similar to `make operator-up` but additionally starts a [skaffold dev loop](https://skaffold.dev/docs/workflows/dev/).
After the initial deployment, skaffold starts watching source files.
Once it has detected changes, press any key to trigger a new build and deployment of the changed components.

### Debugging Gardener Operator (Optional)

```shell
make operator-debug
```

This is similar to `make gardener-debug` but for Gardener Operator component. Please check [Debugging Gardener](../deployment/getting_started_locally.md#debugging-gardener) for details.

### Creating a `Garden`

In order to create a garden, just run:

```shell
kubectl apply -f example/operator/20-garden.yaml
```

You can wait for the `Garden` to be ready by running:

```shell
./hack/usage/wait-for.sh garden local Reconciled
```

Alternatively, you can run `kubectl get garden` and wait for the `RECONCILED` status to reach `True`:

```shell
NAME     RECONCILED    AGE
garden   Progressing   1s
```

(Optional): Instead of creating above `Garden` resource manually, you could execute the e2e tests by running:

```shell
make test-e2e-local-operator
```

#### Accessing the Virtual Garden Cluster

⚠️ Please note that in this setup, the virtual garden cluster is not accessible by default when you download the kubeconfig and try to communicate with it.
The reason is that your host most probably cannot resolve the DNS name of the cluster.
Hence, if you want to access the virtual garden cluster, you have to run the following command which will extend your `/etc/hosts` file with the required information to make the DNS names resolvable:

```shell
cat <<EOF | sudo tee -a /etc/hosts

# Manually created to access local Gardener virtual garden cluster.
# TODO: Remove this again when the virtual garden cluster access is no longer required.
127.0.0.1 api.virtual-garden.local.gardener.cloud
EOF
```

To access the virtual garden, you can acquire a `kubeconfig` by

```shell
kubectl -n garden get secret gardener -o jsonpath={.data.kubeconfig} | base64 -d > /tmp/virtual-garden-kubeconfig
kubectl --kubeconfig /tmp/virtual-garden-kubeconfig get namespaces
```

Note that this kubeconfig uses a token that has validity of `12h` only, hence it might expire and causing you to re-download the kubeconfig.

### Deleting the `Garden`

```shell
./hack/usage/delete garden local
```

### Tear Down the Gardener Operator Environment

```shell
make operator-down
make kind-operator-down
```
