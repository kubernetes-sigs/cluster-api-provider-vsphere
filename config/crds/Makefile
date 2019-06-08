# Copyright 2019 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

all: build

API_VERSION := v1alpha1
CLUSTER_CRD := vsphereproviderconfig_$(API_VERSION)_vsphereclusterproviderconfig.yaml
MACHINE_CRD := vsphereproviderconfig_$(API_VERSION)_vspheremachineproviderconfig.yaml
CONTROLLER_GEN := $(abspath ../../vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go)
OUTPUT_DIR ?= $(abspath .)
VERIFY_DIR ?= $(abspath .verify)
VERIFY_LOG := $(VERIFY_DIR)/log

# If this is running on Prow, do not build the CRDs, only assert that CRDs
# generated from the commit should match the CRDs that are a part of the
# commit.
build: MAKEFLAGS += --no-print-directory
build:
ifneq (,$(PROW_JOB_ID))
	$(MAKE) verify
else
	$(MAKE) generate
endif
.PHONY: build

generate:
	@rm -fr $(VERIFY_DIR)
	cd ../.. && go run $(CONTROLLER_GEN) crd --output-dir $(OUTPUT_DIR)

verify: OUTPUT_DIR=$(VERIFY_DIR)
verify: generate
	@diff $(CLUSTER_CRD) $(VERIFY_DIR)/$(CLUSTER_CRD) >>$(VERIFY_LOG) 2>&1 || true
	@diff $(MACHINE_CRD) $(VERIFY_DIR)/$(MACHINE_CRD) >>$(VERIFY_LOG) 2>&1 || true
	@if [ -f $(VERIFY_LOG) ] && [ ! "$$(wc -l $(VERIFY_LOG) | awk '{print $$1}')" = "0" ]; then \
	  echo "CRDs do not match with already generated CRDs" 1>&2; \
	  cat $(VERIFY_LOG); exit 1; \
	fi
.PHONY: validate

clean:
	rm -fr $(CLUSTER_CRD) $(MACHINE_CRD) $(VERIFY_DIR)
.PHONY: clean
