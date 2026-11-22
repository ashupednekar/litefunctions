PY ?= python3

.PHONY: test dev fmt clean local-push

test:
	$(PY) -m unittest discover -s tests -v

dev:
	$(PY) tools/dev_server.py

fmt:
	$(PY) -m compileall -q functions/python/invoices tools tests

clean:
	find . -type d -name __pycache__ -prune -exec rm -rf {} +
	rm -rf .ruff_cache

# Deploy to the local cluster: PROJECT=one make local-push
local-push:
	bash scripts/local-push.sh
