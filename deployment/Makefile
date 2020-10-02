.PHONY: check_delete_operator delete_operator create_new_operator build_operator_image push_operator 
.PHONY: check_delete_csv delete_csv create_new_csv 
.PHONY: clean_bundle build_operator_bundle_image push_operator_bundle
.PHONY: clean_catalog build_operator_catalog_image push_operator_catalog

CAT_CMD=$(if $(filter $(OS),Windows_NT),type,cat)
OPERATOR_VERSION_FILE=./OPERATOR_VERSION
OPERATOR_VER=v$(shell $(CAT_CMD) $(OPERATOR_VERSION_FILE))
OPERATOR_VER_NUM=$(shell $(CAT_CMD) $(OPERATOR_VERSION_FILE))
CONTROLLER_VERSION_FILE=../CONTROLLER_VERSION
CONTROLLER_VER=v$(shell $(CAT_CMD) $(CONTROLLER_VERSION_FILE))

check_delete_operator:
	@echo "****************************************************************************************************"
	@echo "* WARNING: This operation will destroy the operator and generate a completely new operator which   *"
	@echo "*          may require manual patches.  This option should only be done by advanced users.         *" 
	@echo "*          Optionally, to update this operator use other targets such as 'build_operator_image'.   *"
	@echo "****************************************************************************************************"
	@echo "Continue? [y/N] " && read ans && [ $${ans:-N} == y ] 

delete_operator: check_delete_operator
	rm -rf mcad-operator

create_new_operator: delete_operator
	operator-sdk-v0.18.1 new mcad-operator --crd-version=v1beta1 --api-version=mcad.ibm.com/v1beta1 --kind=MCADHelmConfig --type=helm --helm-chart=./mcad-controller

build_operator_image:
	sed -i '' "s|image: REPLACE_IMAGE|image: darroyo/mcad-operator:${OPERATOR_VER}|" mcad-operator/deploy/operator.yaml
	sed -i '' "s|image: darroyo/mcad-operator:v.\..\..|image: darroyo/mcad-operator:${OPERATOR_VER}|" mcad-operator/deploy/operator.yaml
	sed -i '' "s|repository: mcad-controller|repository: darroyo/mcad-controller|" mcad-operator/helm-charts/mcad-controller/values.yaml
	sed -i '' "s|tag: latest|tag: ${CONTROLLER_VER}|" mcad-operator/helm-charts/mcad-controller/values.yaml
	sed -i '' "s|tag: v.\...\..|tag: ${CONTROLLER_VER}|" mcad-operator/helm-charts/mcad-controller/values.yaml
	sed -i '' "s|repository: mcad-controller|repository: darroyo/mcad-controller|" mcad-operator/deploy/crds/mcad.ibm.com_v1beta1_mcadhelmconfig_cr.yaml
	sed -i '' "s|tag: latest|tag: ${CONTROLLER_VER}|" mcad-operator/deploy/crds/mcad.ibm.com_v1beta1_mcadhelmconfig_cr.yaml
	sed -i '' "s|tag: v.\...\..|tag: ${CONTROLLER_VER}|" mcad-operator/deploy/crds/mcad.ibm.com_v1beta1_mcadhelmconfig_cr.yaml
	cd ./mcad-operator && operator-sdk-v0.18.1 build --image-build-args --no-cache darroyo/mcad-operator:${OPERATOR_VER}

push_operator: build_operator_image
	docker push darroyo/mcad-operator:${OPERATOR_VER}

check_delete_csv:
	@echo "******************************************************************************************************"
	@echo "* WARNING: This operation will destroy the 'csv' file and generate a completely new 'csv' file which *"
	@echo "*          may require manual patches.  This option should only be done by advanced users.           *" 
	@echo "*          Optionally, to update this operator use other targets such as 'build_operator_image'.     *"
	@echo "******************************************************************************************************"
	@echo "Continue? [y/N] " && read ans && [ $${ans:-N} == y ] 

delete_csv: check_delete_csv
	if [ -d "./mcad-operator/deploy/olm-catalog" ]; then cd ./mcad-operator/deploy && rm -rf olm-catalog; fi

# Useful links:
# https://docs.openshift.com/container-platform/4.5/operators/operator_sdk/osdk-helm.html
# https://docs.openshift.com/container-platform/4.5/operators/operator_sdk/osdk-working-bundle-images.html#osdk-working-bundle-images
# https://docs.openshift.com/container-platform/4.5/operators/olm-managing-custom-catalogs.html#olm-managing-custom-catalogs-bundle-format
# https://operatorhub.io/preview
# https://operatorhub.io/packages

create_new_csv: delete_csv
	cd ./mcad-operator && operator-sdk-v0.18.1 generate csv --csv-version ${OPERATOR_VER_NUM}

clean_bundle:
	if [ -d "./mcad-operator/bundle" ]; then cd ./mcad-operator && rm -rf bundle; fi
	if [ -f "./mcad-operator/bundle.Dockerfile" ]; then cd ./mcad-operator && rm bundle.Dockerfile; fi

build_operator_bundle_image: clean_bundle
	cd ./mcad-operator && operator-sdk-v0.18.1 bundle create --generate-only -o ./bundle
	docker build --no-cache -f ./mcad-operator/bundle.Dockerfile -t darroyo/mcad-operator-bundle:${OPERATOR_VER} ./mcad-operator

push_operator_bundle: build_operator_bundle_image
	docker push darroyo/mcad-operator-bundle:${OPERATOR_VER}
	operator-sdk-v0.18.1 bundle validate -b docker --verbose  darroyo/mcad-operator-bundle:${OPERATOR_VER}

clean_catalog:
	if [ -d "./mcad-operator/database" ]; then cd ./mcad-operator && rm -rf database; fi
	if [ -f "./mcad-operator/catalog.Dockerfile" ]; then cd ./mcad-operator && rm catalog.Dockerfile; fi

build_operator_catalog_image: clean_catalog
	cd ./mcad-operator && opm-v1.13.8 index add -c docker --bundles darroyo/mcad-operator-bundle:${OPERATOR_VER} --generate -d catalog.Dockerfile
	docker build --no-cache -f ./mcad-operator/catalog.Dockerfile -t darroyo/mcad-operator-catalog:${OPERATOR_VER} ./mcad-operator

push_operator_catalog: build_operator_catalog_image
	docker push darroyo/mcad-operator-catalog:${OPERATOR_VER}
