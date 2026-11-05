.PHONY: demo cluster deploy break wait-failed inspect delete

CLUSTER ?= kubehealth-demo
CONTEXT := kind-$(CLUSTER)
NAMESPACE ?= default
DEPLOYMENT ?= kubehealth-demo

demo:
	$(MAKE) cluster
	$(MAKE) deploy
	$(MAKE) break
	$(MAKE) wait-failed
	$(MAKE) inspect

cluster:
	@if kind get clusters 2>/dev/null | grep -Fxq "$(CLUSTER)"; then \
		echo "kind cluster $(CLUSTER) already exists"; \
	else \
		kind create cluster --name "$(CLUSTER)" --wait 60s; \
	fi

deploy:
	kubectl --context "$(CONTEXT)" apply -n "$(NAMESPACE)" -f deployment.yaml
	kubectl --context "$(CONTEXT)" rollout status -n "$(NAMESPACE)" deployment/$(DEPLOYMENT) --timeout=120s

break:
	kubectl --context "$(CONTEXT)" set image -n "$(NAMESPACE)" deployment/$(DEPLOYMENT) nginx=nginx:kubehealth-image-does-not-exist

wait-failed:
	@attempt=0; \
	while [ $$attempt -lt 90 ]; do \
		reason=$$(kubectl --context "$(CONTEXT)" get -n "$(NAMESPACE)" deployment/$(DEPLOYMENT) -o jsonpath='{.status.conditions[?(@.type=="Progressing")].reason}'); \
		if [ "$$reason" = "ProgressDeadlineExceeded" ]; then \
			echo "Deployment rollout failed as expected"; \
			exit 0; \
		fi; \
		attempt=$$((attempt + 1)); \
		sleep 1; \
	done; \
	echo "Timed out waiting for ProgressDeadlineExceeded" >&2; \
	exit 1

inspect:
	KUBE_CONTEXT="$(CONTEXT)" go run . -namespace "$(NAMESPACE)" -name "$(DEPLOYMENT)"

delete:
	kind delete cluster --name "$(CLUSTER)"
