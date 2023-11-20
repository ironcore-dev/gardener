# ETCD Encryption Config

The `spec.kubernetes.kubeAPIServer.encryptionConfig` field in the Shoot API allows users to customize encryption configurations for the API server. It provides options to specify additional resources for encryption beyond secrets and to exclude specific resources from encryption.

## Usage Guidelines

### `resources` field

- This field can be used to specify resources that should be encrypted in addition to secrets. Secrets are always encrypted.
- Each item is a Kubernetes resource name in plural (resource or resource.group) or a wildcard (`*.*` or `*.<group>`) that should be encrypted. Use `*.<group>` to encrypt all resources within a group (for eg `*.apps` in above example) or `*.*` to encrypt all resources. `*.` can be used to encrypt all resource in the core group. `*.*` will encrypt all resources, even custom resources that are added after API server start.
- `*.<group>` and `<resource>.<group>` should not be added together since this can lead to overlapping of resources. Similarly if `*.*` is added, no other item can be added to this list.
- See [Encrypting Confidential Data at Rest
](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data) for more details.
- If resources are added, users need to issue update requests for all existing objects (e.g. empty patches) to encrypt the data in etcd.

> :warning: Resources can only be added to this list, no resources can be removed. This is because when the kubernetes API server is restarted with the new config, the `identity` provider (See [Available Providers](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/#providers)) will be used to read the resource and it will not be able to read a resource which is already present in the encrypted state. So you should be really careful before adding a wildcard for all resources (`*.*`) or for a whole group (`*.<group>`) since this action cannot be reversed.

### `excludedResources` field

- Opting out of encryption for specific resources while a wildcard is enabled can be achieved by adding a particular resource or a wildcard for a group to this list.
- For example, if `*.*` is enabled and you want to opt-out encryption for the `apps` group, you can add `*.apps` in the excluded resources. Or if you want to enable encryption for `*.customoperator.io`, and you want to opt-out of encryption for the resource `events`, you can do so by adding `events.customoperator.io` to the excluded resources.
- Note that items can only be added to this list before the `encryptionConfig.resources` contains a wildcard for it. Otherwise the resource will already be encrypted and hence cannot be excluded. For example, if `*.apps` is enabled already, then adding `deployments.apps` to the excluded list is not allowed.
- However, removing items from this list is allowed. If resources are removed, users need to issue update requests for all existing objects (e.g. empty patches) to encrypt the data in etcd.

> ℹ️ Note that wildcards are only supported for Kubernetes versions >= v1.27 and configuring encryption for a custom resource is only supported for  versions >= 1.26.

## Example Usage in a `Shoot`

```yaml
spec:
  kubernetes:
    kubeAPIServer:
      encryptionConfig:
        excludedResources:
          - deployments.apps
          - random.fancyresource.io
        resources:
          - configmaps
          - '*.apps'
          - customresource.fancyoperator.io
          - '*.fancyresource.io'
```
