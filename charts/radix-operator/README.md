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
