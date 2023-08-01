BIN_DIR=_output/bin
CAT_CMD=$(if $(filter $(OS),Windows_NT),type,cat)
RELEASE_VER:=
CURRENT_DIR=$(shell pwd)
GIT_BRANCH:=$(shell git symbolic-ref --short HEAD 2>&1 | grep -v fatal)
#define the GO_BUILD_ARGS if you need to pass additional arguments to the go build
GO_BUILD_ARGS?=

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Versions
CONTROLLER_TOOLS_VERSION ?= v0.9.2
CODEGEN_VERSION ?= v0.20.15

## Tool Binaries
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
APPLYCONFIGURATION_GEN ?= $(LOCALBIN)/applyconfiguration-gen
CLIENT_GEN ?= $(LOCALBIN)/client-gen
LISTER_GEN ?= $(LOCALBIN)/lister-gen
INFORMER_GEN ?= $(LOCALBIN)/informer-gen

# Reset branch name if this a Travis CI environment
ifneq ($(strip $(TRAVIS_BRANCH)),)
	GIT_BRANCH:=${TRAVIS_BRANCH}
endif

TAG:=$(shell echo "")
# Check for git repository id sent by Travis-CI
ifneq ($(strip $(git_repository_id)),)
	TAG:=${TAG}${git_repository_id}-
endif

# Check for current branch name and update 'RELEASE_VER' and 'TAG'
ifneq ($(strip $(GIT_BRANCH)),)
	RELEASE_VER:= $(shell git describe --tags --abbrev=0)
	TAG:=${TAG}${GIT_BRANCH}
	# replace invalid characters that might exist in the branch name
	TAG:=$(shell echo ${TAG} | sed 's/[^a-zA-Z0-9]/-/g')
	TAG:=${TAG}-${RELEASE_VER}
endif

.PHONY: print-global-variables

# Build the controller executable for use in docker image build
mcad-controller: init generate-code
ifeq ($(strip $(GO_BUILD_ARGS)),)
	$(info Compiling controller)
	CGO_ENABLED=0 go build -o ${BIN_DIR}/mcad-controller ./cmd/kar-controllers/
else
	$(info Compiling controller with build arguments: '${GO_BUILD_ARGS}')
	go build $(GO_BUILD_ARGS) -o ${BIN_DIR}/mcad-controller ./cmd/kar-controllers/
endif	

print-global-variables:
	$(info "---")
	$(info "MAKE GLOBAL VARIABLES:")
	$(info "  "BIN_DIR="$(BIN_DIR)")
	$(info "  "GIT_BRANCH="$(GIT_BRANCH)")
	$(info "  "RELEASE_VER="$(RELEASE_VER)")
	$(info "  "TAG="$(TAG)")
	$(info "  "GO_BUILD_ARGS="$(GO_BUILD_ARGS)")
	$(info "---")

verify: generate-code
#	hack/verify-gofmt.sh
#	hack/verify-golint.sh
#	hack/verify-gencode.sh

init:
	mkdir -p ${BIN_DIR}

verify-tag-name: print-global-variables
	# Check for invalid tag name
	t=${TAG} && [ $${#t} -le 128 ] || { echo "Target name $$t has 128 or more chars"; false; }
.PHONY: generate-client ## Generate client packages
generate-client: code-generator
	rm -rf pkg/client/clientset/versioned pkg/client/informers/externalversions pkg/client/listers/controller/v1beta1
# TODO: add this back when the version of the tool has been updated and supports this executable
#	$(APPLYCONFIGURATION_GEN) \
#		--input-dirs="github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1" \
#		--go-header-file="hack/boilerplate/boilerplate.go.txt" \
#		--output-package="github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/applyconfiguration" \
#		--trim-path-prefix "github.com/project-codeflare/multi-cluster-app-dispatcher"
	$(CLIENT_GEN) \
 		--input="pkg/apis/controller/v1beta1" \
 		--input-base="github.com/project-codeflare/multi-cluster-app-dispatcher" \
 		--go-header-file="hack/boilerplate/boilerplate.go.txt" \
 		--clientset-name "versioned"  \
 		--output-package="github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/clientset" \
 		--output-base="." 
# TODO: add the following line back once the tool has been upgraded		
# 		--trim-path-prefix "github.com/project-codeflare/multi-cluster-app-dispatcher"
	$(LISTER_GEN) \
 		--input-dirs="github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1" \
 		--go-header-file="hack/boilerplate/boilerplate.go.txt" \
 		--output-base="." \
 		--output-package="github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/listers" 
# TODO: add the following line back once the tool has been upgraded		
# 		--trim-path-prefix "github.com/project-codeflare/multi-cluster-app-dispatcher"
	$(INFORMER_GEN) \
 		--input-dirs="github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1" \
 		--versioned-clientset-package="github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/clientset/versioned" \
 		--listers-package="github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/listers" \
 		--go-header-file="hack/boilerplate/boilerplate.go.txt" \
 		--output-base="." \
 		--output-package="github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/informers" 
# TODO: add the following line back once the tool has been upgraded		
# 		--trim-path-prefix "github.com/project-codeflare/multi-cluster-app-dispatcher"
# TODO: remove the following lines once the tool has been upgraded and they are no longer needed.
# The `mv` and `rm` are necessary as the generators write to the gihub.com/... path.	
	mv -f github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/clientset/versioned pkg/client/clientset/versioned
	mv -f github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/informers/externalversions pkg/client/informers/externalversions
	mv -f github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/listers/controller/v1beta1 pkg/client/listers/controller/v1beta1
	rm -rf github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: code-generator
#TODO: add $(APPLYCONFIGURATION_GEN) as a dependency when the tool is supported
code-generator: $(CLIENT_GEN) $(LISTER_GEN) $(INFORMER_GEN) $(CONTROLLER_GEN)

# TODO: enable this target once the tools is supported
#.PHONY: applyconfiguration-gen
#applyconfiguration-gen: $(APPLYCONFIGURATION_GEN) 
#$(APPLYCONFIGURATION_GEN): $(LOCALBIN)
#	test -s $(LOCALBIN)/applyconfiguration-gen || GOBIN=$(LOCALBIN) go install k8s.io/code-generator/cmd/applyconfiguration-gen@$(CODEGEN_VERSION)

.PHONY: client-gen
client-gen: $(CLIENT_GEN)
$(CLIENT_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/client-gen || GOBIN=$(LOCALBIN) go install k8s.io/code-generator/cmd/client-gen@$(CODEGEN_VERSION)

.PHONY: lister-gen
lister-gen: $(LISTER_GEN)
$(LISTER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/lister-gen || GOBIN=$(LOCALBIN) go install k8s.io/code-generator/cmd/lister-gen@$(CODEGEN_VERSION)

.PHONY: informer-gen
informer-gen: $(INFORMER_GEN)
$(INFORMER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/informer-gen || GOBIN=$(LOCALBIN) go install k8s.io/code-generator/cmd/informer-gen@$(CODEGEN_VERSION)	

.PHONY: manifests
manifests: controller-gen ## Generate CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd:allowDangerousTypes=true paths="./pkg/apis/..." output:crd:artifacts:config=config/crd/bases

generate-code: pkg/apis/controller/v1beta1/zz_generated.deepcopy.go

pkg/apis/controller/v1beta1/zz_generated.deepcopy.go: ${BIN_DIR}/deepcopy-gen
	$(info Generating deepcopy...)
	${BIN_DIR}/deepcopy-gen -i ./pkg/apis/controller/v1beta1/ -O zz_generated.deepcopy 
	${BIN_DIR}/deepcopy-gen -i ./pkg/apis/quotaplugins/quotasubtree/v1 -O zz_generated.deepcopy

${BIN_DIR}/deepcopy-gen:	
	$(info Compiling deepcopy-gen...)
	go build -o ${BIN_DIR}/deepcopy-gen ./cmd/deepcopy-gen/

images: verify-tag-name generate-code update-deployment-crds
	$(info List executable directory)
	$(info repo id: ${git_repository_id})
	$(info branch: ${GIT_BRANCH})
	$(info Build the docker image)
ifeq ($(strip $(GO_BUILD_ARGS)),)
	docker build --quiet --no-cache --tag mcad-controller:${TAG} -f ${CURRENT_DIR}/Dockerfile  ${CURRENT_DIR}
else 
	docker build --no-cache --tag mcad-controller:${TAG} --build-arg GO_BUILD_ARGS=$(GO_BUILD_ARGS) -f ${CURRENT_DIR}/Dockerfile  ${CURRENT_DIR}
endif		

images-podman: verify-tag-name generate-code update-deployment-crds
	$(info List executable directory)
	$(info repo id: ${git_repository_id})
	$(info branch: ${GIT_BRANCH})
	$(info Build the docker image)
ifeq ($(strip $(GO_BUILD_ARGS)),)
	podman build --quiet --no-cache --tag mcad-controller:${TAG} -f ${CURRENT_DIR}/Dockerfile  ${CURRENT_DIR}
else
	podman build --no-cache --tag mcad-controller:${TAG} --build-arg GO_BUILD_ARGS=$(GO_BUILD_ARGS) -f ${CURRENT_DIR}/Dockerfile  ${CURRENT_DIR}
endif	

push-images: verify-tag-name
ifeq ($(strip $(quay_repository)),)
	$(info No registry information provided.  To push images to a docker registry please set)
	$(info environment variables: quay_repository, quay_token, and quay_id.  Environment)
	$(info variables do not need to be set for github Travis CICD.)
else
	$(info Log into quay)
	docker login quay.io -u ${quay_id} --password ${quay_token}
	$(info Tag the latest image)
	docker tag mcad-controller:${TAG}  ${quay_repository}/mcad-controller:${TAG}
	$(info Push the docker image to registry)
	docker push ${quay_repository}/mcad-controller:${TAG}
ifeq ($(strip $(git_repository_id)),main)
	$(info Update the `latest` tag when built from `main`)
	docker tag mcad-controller:${TAG}  ${quay_repository}/mcad-controller:latest
	docker push ${quay_repository}/mcad-controller:latest
endif
ifneq ($(TAG:release-v%=%),$(TAG))
	$(info Update the `stable` tag to point `latest` release image)
	docker tag mcad-controller:${TAG} ${quay_repository}/mcad-controller:stable
	docker push ${quay_repository}/mcad-controller:stable
endif
endif

run-test:
	$(info Running unit tests...)
	go test -v -coverprofile cover.out -race -parallel 8  ./pkg/...

run-e2e: verify-tag-name update-deployment-crds
ifeq ($(strip $(quay_repository)),)
	echo "Running e2e with MCAD local image: mcad-controller ${TAG} IfNotPresent."
	hack/run-e2e-kind.sh mcad-controller ${TAG} IfNotPresent
else
	echo "Running e2e with MCAD registry image image: ${quay_repository}/mcad-controller ${TAG}."
	hack/run-e2e-kind.sh ${quay_repository}/mcad-controller ${TAG}
endif

coverage:
#	KUBE_COVER=y hack/make-rules/test.sh $(WHAT) $(TESTS)

clean:
	rm -rf _output/

#CRD file maintenance rules
DEPLOYMENT_CRD_DIR=deployment/mcad-controller/crds
CRD_BASE_DIR=config/crd/bases
MCAD_CRDS= ${DEPLOYMENT_CRD_DIR}/ibm.com_quotasubtrees.yaml  \
		   ${DEPLOYMENT_CRD_DIR}/mcad.ibm.com_appwrappers.yaml \
		   ${DEPLOYMENT_CRD_DIR}/mcad.ibm.com_queuejobs.yaml \
		   ${DEPLOYMENT_CRD_DIR}/mcad.ibm.com_schedulingspecs.yaml

update-deployment-crds: ${MCAD_CRDS}

${DEPLOYMENT_CRD_DIR}/ibm.com_quotasubtrees.yaml : ${CRD_BASE_DIR}/ibm.com_quotasubtrees.yaml
${DEPLOYMENT_CRD_DIR}/mcad.ibm.com_appwrappers.yaml : ${CRD_BASE_DIR}/mcad.ibm.com_appwrappers.yaml
${DEPLOYMENT_CRD_DIR}/mcad.ibm.com_queuejobs.yaml : ${CRD_BASE_DIR}/mcad.ibm.com_queuejobs.yaml
${DEPLOYMENT_CRD_DIR}/mcad.ibm.com_schedulingspecs.yaml : ${CRD_BASE_DIR}/mcad.ibm.com_schedulingspecs.yaml

$(DEPLOYMENT_CRD_DIR)/%: ${CRD_BASE_DIR}/%
	cp $< $@
