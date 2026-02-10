helm repo add istio https://istio-release.storage.googleapis.com/charts
helm repo update
echo "added istio chart"

kubectl create namespace istio-system
echo "created istio-system namespace"

helm install istio-base istio/base -n istio-system --wait
echo "installed istio base"

helm install istiod istio/istiod -n istio-system --wait
echo "installed istio istiod"

helm install istio-ingress istio/gateway -n istio-ingress --create-namespace --wait
echo "installed istio ingress"
