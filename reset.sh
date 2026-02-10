colima kubernetes reset
./setup_istio.sh
kubectl create secret generic litefunctions-ngrok-secret --from-literal=token=$NGROK_TOKEN
