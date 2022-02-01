# Kube Nukem - Nuke a CRD from your Cluster

This is a very simple program that is useful for removing a CRD and all resources of it from
a cluster. For this the CRD will be deleted and then any remaining finalizers will be removed
from stuck resources. Assuming no owner references block the deletion, this gets rid of all
resources.

```
$ kubenukem mytestcrd.omnicorp.com
```

## Installation

You need Go 1.17 installed on your machine.

```
go get go.xrstf.de/kubenukem
```

## Usage

```
Usage of ./kubenukem:
  -kubeconfig string
        kubeconfig file to use
  -verbose
        enable more verbose logging
```

The kubeconfig can also be given using the `KUBECONFIG` environment variable.

## License

MIT
