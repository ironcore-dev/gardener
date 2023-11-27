# ETCD Encryption Config

The `spec.kubernetes.kubeAPIServer.encryptionConfig` field in the Shoot API allows users to customize encryption configurations for the API server. It provides options to specify additional resources for encryption beyond secrets.

## Usage Guidelines

### `resources` field

- This field can be used to specify resources that should be encrypted in addition to secrets. Secrets are always encrypted.
- Each item is a Kubernetes resource name in plural (resource or resource.group). Wild cards are not supported.
- Adding a resource to this list will cause empty patch requests for all added resources to encrypt them in the etcd. See [Encrypting Confidential Data at Rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data) for more details.
- Removing a resource from this list will cause empty patch requests for all removed resources to decrypt and rewrite the resource as plain text. See [Decrypt Confidential Data that is Already Encrypted at Rest](https://kubernetes.io/docs/tasks/administer-cluster/decrypt-data/) for more details.

> ℹ️ Note that configuring encryption for a custom resource is only supported for  versions >= 1.26.

## Example Usage in a `Shoot`

```yaml
spec:
  kubernetes:
    kubeAPIServer:
      encryptionConfig:
        resources:
          - configmaps
          - customresource.fancyoperator.io
```
