# Migration note

> [!IMPORTANT]
> Kube Nukem has been migrated to [codeberg.org/xrstf/kubenukem](https://codeberg.org/xrstf/kubenukem).

---

# Kube Nukem - Nuke a CRD from your Cluster

This is a very simple program that is useful for removing a CRD and all resources of it from
a cluster. For this the CRD will be deleted and then any remaining finalizers will be removed
from stuck resources. Assuming no owner references block the deletion, this gets rid of all
resources.

```bash
$ kubenukem mytestcrd.omnicorp.com
```

## Installation

You need Go 1.20 installed on your machine.

```bash
go install go.xrstf.de/kubenukem
```

## Usage

```
Usage of _build/kubenukem:
      --kubeconfig string   kubeconfig file to use (uses $KUBECONFIG by default)
  -v, --verbose             enable more verbose logging
  -V, --version             show version info and exit immediately
```

The kubeconfig can also be given using the `KUBECONFIG` environment variable.

## License

MIT
