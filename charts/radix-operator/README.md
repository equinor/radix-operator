# Updating helm chart

1. Push updated `helm` chart to ACR. **PS: See note below if you have not used private ACR Helm Repos before.**

```
cd charts/
export CHART_VERSION=`cat radix-operator/Chart.yaml | yq --raw-output .version`
tar -zcvf radix-operator-$CHART_VERSION.tgz radix-operator
az acr helm push --name radixdev radix-operator-$CHART_VERSION.tgz
az acr helm push --name radixprod radix-operator-$CHART_VERSION.tgz
```

> Uses `yq` to extract version from `Charts.yaml`. Install with `sudo apt-get install jq && pip install yq`

# Flux deployment

As of mid April 2019, we began a journey in automating the `radix-operator` helm chart by using a tool called `Flux` (https://github.com/weaveworks/flux).

In order for the sync to work, we currently have to sync the `radix-operator` helm chart with the one in `radix-flux` repo (https://github.com/equinor/radix-flux). That means any change in the `radix-operator` helm chart in this repo should be copied *manually* to the helm chart in the `radix-flux` repo.