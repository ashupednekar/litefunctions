"""Render regression checks. Run: python3 -m unittest discover -s chart/tests.

Requires Helm, PyYAML, and chart dependencies (helm dependency build chart).
"""

from pathlib import Path
import subprocess
import unittest

import yaml


CHART = Path(__file__).resolve().parents[1]


def render(*settings, release="litefunctions", success=True):
    command = ["helm", "template", release, str(CHART), "--namespace", "litefunctions"]
    for setting in settings:
        command.extend(["--set", setting])
    result = subprocess.run(command, capture_output=True, text=True)
    if not success:
        assert result.returncode != 0, "Expected invalid configuration to fail"
        return result.stderr
    assert result.returncode == 0, result.stderr
    documents = [doc for doc in yaml.safe_load_all(result.stdout) if doc]
    assert not any(doc["kind"] == "Ingress" for doc in documents)
    return documents


def resources(documents, kind):
    return {doc["metadata"]["name"]: doc for doc in documents if doc["kind"] == kind}


class GatewayTests(unittest.TestCase):
    def test_default_routes(self):
        docs = render()
        routes = resources(docs, "HTTPRoute")
        self.assertEqual(set(routes), {"litefunctions-portal", "litefunctions-ingestor", "litefunctions-gitea"})
        self.assertEqual(routes["litefunctions-ingestor"]["spec"]["rules"][0]["matches"][0]["path"]["value"], "/lambda")
        self.assertEqual(routes["litefunctions-portal"]["spec"]["rules"][0]["backendRefs"], [{"name": "litefunctions-portal", "port": 3000}])

    def test_disabled_gateway(self):
        docs = render("gatewayApi.enabled=false")
        for kind in ["Gateway", "HTTPRoute", "Certificate", "Issuer", "ClusterIssuer"]:
            self.assertFalse(resources(docs, kind))

    def test_disabled_components(self):
        docs = render("ui.enabled=false", "gitea.enabled=false")
        self.assertEqual(set(resources(docs, "HTTPRoute")), {"litefunctions-ingestor"})
        gateway = next(iter(resources(docs, "Gateway").values()))
        self.assertFalse(any("gitea" in listener["hostname"] for listener in gateway["spec"]["listeners"]))

    def test_rustfs_backends_and_tls(self):
        docs = render("rustfs.enabled=true", "rustfs.fullnameOverride=storage", "rustfs.service.endpoint.port=9100", "rustfs.service.console.port=9101", "gitea.fullnameOverride=git", "gitea.service.http.port=3100", "gatewayApi.hosts.rustfsConsole[0]=console.example.com", release="custom")
        routes = resources(docs, "HTTPRoute")
        for name, port in [("litefunctions-rustfs", 9100), ("litefunctions-rustfs-console", 9101)]:
            self.assertEqual(routes[name]["spec"]["rules"][0]["backendRefs"], [{"name": "storage-svc", "port": port}])
        self.assertEqual(routes["litefunctions-gitea"]["spec"]["rules"][0]["backendRefs"], [{"name": "git-http", "port": 3100}])
        services = resources(docs, "Service")
        self.assertIn("storage-svc", services)
        hosts = {host for route in routes.values() for host in route["spec"]["hostnames"]}
        gateway = next(iter(resources(docs, "Gateway").values()))
        self.assertEqual(hosts, {listener["hostname"] for listener in gateway["spec"]["listeners"]})
        cert = next(iter(resources(docs, "Certificate").values()))
        self.assertEqual(hosts, set(cert["spec"]["dnsNames"]))

    def test_http_only_and_external_certificate(self):
        docs = render("gatewayApi.tls.enabled=false")
        gateway = next(iter(resources(docs, "Gateway").values()))
        self.assertTrue(all(listener["protocol"] == "HTTP" for listener in gateway["spec"]["listeners"]))
        self.assertFalse(resources(docs, "Certificate"))
        docs = render("gatewayApi.tls.certManager.enabled=false", "gatewayApi.tls.secretName=existing-tls")
        self.assertFalse(resources(docs, "Certificate"))
        gateway = next(iter(resources(docs, "Gateway").values()))
        for listener in gateway["spec"]["listeners"]:
            if listener["protocol"] == "HTTPS":
                self.assertEqual(listener["tls"]["certificateRefs"][0]["name"], "existing-tls")

    def test_legacy_values_rejected(self):
        for name in ["ui", "gitea", "gitea.gitea", "rustfs"]:
            with self.subTest(name=name):
                self.assertIn("deprecated", render(f"{name}.ingress.enabled=true", success=False))

    def test_acme(self):
        settings = ["gatewayApi.tls.certManager.selfSigned.enabled=false", "gatewayApi.tls.certManager.letsencrypt.enabled=true", "gatewayApi.tls.certManager.letsencrypt.email=admin@example.com", "gatewayApi.hosts.portal[0]=portal.example.com", "gatewayApi.hosts.gitea[0]=git.example.com"]
        self.assertIn("port=80", render(*settings, success=False))
        docs = render(*settings, "gatewayApi.http.port=80")
        issuer = next(iter(resources(docs, "ClusterIssuer").values()))
        self.assertEqual(issuer["spec"]["acme"]["solvers"][0]["http01"]["gatewayHTTPRoute"]["parentRefs"][0]["name"], "litefunctions-gateway")
        configs = resources(docs, "ConfigMap")
        config = yaml.safe_load(configs["litefunctions-cert-manager"]["data"]["config.yaml"])
        self.assertTrue(config["enableGatewayAPI"])

    def test_ngrok(self):
        docs = render("ngrok.enabled=true")
        deployment = resources(docs, "Deployment")["litefunctions-ngrok"]
        self.assertIn("litefunctions-gateway-istio.litefunctions.svc:8080", deployment["spec"]["template"]["spec"]["containers"][0]["args"])
        self.assertIn("requires gatewayApi.enabled", render("ngrok.enabled=true", "gatewayApi.enabled=false", success=False))
