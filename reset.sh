colima kubernetes reset || echo "do this in rancher desktop"
./setup_istio.sh
LB_IP=$(kubectl -n istio-ingress get svc istio-ingress -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
echo "$LB_IP litefunctions.portal litefunctions.gitea" | sudo tee -a /etc/hosts
mkcert -cert-file /tmp/litefunctions-local.pem -key-file /tmp/litefunctions-local-key.pem litefunctions.portal litefunctions.gitea
kubectl -n istio-ingress create secret tls litefunctions-local-tls --cert=/tmp/litefunctions-local.pem --key=/tmp/litefunctions-local-key.pem --dry-run=client -o yaml | kubectl apply -f -
mkcert -install
helm install litefunctions chart --debug
