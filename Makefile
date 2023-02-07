BIN_DIR=_output/bin
CAT_CMD=$(if $(filter $(OS),Windows_NT),type,cat)
VERSION_FILE=./CONTROLLER_VERSION
RELEASE_VER=v$(shell $(CAT_CMD) $(VERSION_FILE))
CURRENT_DIR=$(shell pwd)
GIT_BRANCH:=$(shell git symbolic-ref --short HEAD 2>&1 | grep -v fatal)
# Reset branch name if this a Travis CI environment
ifneq ($(strip $(TRAVIS_BRANCH)),)
	GIT_BRANCH:=${TRAVIS_BRANCH}
endif

TAG:=$(shell echo "")
# Check for git repository id sent by Travis-CI
ifneq ($(strip $(git_repository_id)),)
	TAG:=${TAG}${git_repository_id}-
endif

# Check for current branch name
ifneq ($(strip $(GIT_BRANCH)),)
	TAG:=${TAG}${GIT_BRANCH}-
endif
TAG:=${TAG}${RELEASE_VER}

.PHONY: print-global-variables

mcad-controller: init generate-code
	$(info Compiling controller)
	CGO_ENABLED=0 GOARCH=amd64 go build -o ${BIN_DIR}/mcad-controller ./cmd/kar-controllers/

print-global-variables:
	$(info "---")
	$(info "MAKE GLOBAL VARIABLES:")
	$(info "  "BIN_DIR="$(BIN_DIR)")
	$(info "  "GIT_BRANCH="$(GIT_BRANCH)")
	$(info "  "RELEASE_VER="$(RELEASE_VER)")
	$(info "  "TAG="$(TAG)")
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

generate-code:
	$(info Compiling deepcopy-gen...)
	go build -o ${BIN_DIR}/deepcopy-gen ./cmd/deepcopy-gen/
	$(info Generating deepcopy...)
	${BIN_DIR}/deepcopy-gen -i ./pkg/apis/controller/v1beta1/ -O zz_generated.deepcopy 

images: verify-tag-name
	$(info List executable directory)
	$(info repo id: ${git_repository_id})
	$(info branch: ${GIT_BRANCH})
	ls -l ${CURRENT_DIR}/_output/bin
	$(info Build the docker image)
	docker build --quiet --no-cache --tag mcad-controller:${TAG} -f ${CURRENT_DIR}/deployment/Dockerfile.both  ${CURRENT_DIR}/_output/bin

images-podman: verify-tag-name
	$(info List executable directory)
	$(info repo id: ${git_repository_id})
	$(info branch: ${GIT_BRANCH})
	ls -l ${CURRENT_DIR}/_output/bin
	$(info Build the docker image)
	podman build --quiet --no-cache --tag mcad-controller:${TAG} -f ${CURRENT_DIR}/deployment/Dockerfile.both  ${CURRENT_DIR}/_output/bin

push-images: verify-tag-name
ifeq ($(strip $(quay_repository)),)
	$(info No registry information provide.  To push images to a docker registry please set)
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
endif

run-test:
	$(info Running unit tests...)
	hack/make-rules/test.sh $(WHAT) $(TESTS)

run-e2e: mcad-controller verify-tag-name
ifeq ($(strip $(quay_repository)),)
	echo "Running e2e with MCAD local image: mcad-controller ${TAG} IfNotPresent."
	hack/run-e2e-kind.sh mcad-controller ${TAG} IfNotPresent
else
	echo "Running e2e with MCAD registry image image: ${quay_repository}/mcad-controller ${TAG}."
	hack/run-e2e-kind.sh ${quay_repository}/mcad-controller ${TAG}
endif

mcad-controller-private: init generate-code
	$(info Compiling controller)
	CGO_ENABLED=0 GOARCH=amd64 GOPRIVATE=github.ibm.com/* go build -tags private -modfile ./private.mod -o ${BIN_DIR}/mcad-controller ./cmd/kar-controllers/

coverage:
#	KUBE_COVER=y hack/make-rules/test.sh $(WHAT) $(TESTS)

clean:
	rm -rf _output/
	rm -f mcad-controllers
