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

# Initialize var.
IMAGE_TAG:=$(shell echo "")
# Check for git repository id sent by Travis-CI
ifneq ($(strip $(git_repository_id)),)
	IMAGE_TAG:=${IMAGE_TAG}${git_repository_id}-
endif

# Check for current branch name
ifneq ($(strip $(GIT_BRANCH)),)
	IMAGE_TAG:=${IMAGE_TAG}${GIT_BRANCH}-
endif
IMAGE_TAG:=${IMAGE_TAG}${RELEASE_VER}

# Initialize var.
IMAGE_TAG_OPT:=$(shell echo "")
# Set optional go build tag parameter from env. var
ifneq ($(strip $(BUILD_TAG)),)
	IMAGE_TAG_OPT:=-tags ${BUILD_TAG}
endif

# Set GOPRIVATE env. var for go build if provided
ifneq ($(strip $(BUILD_GOPRIVATE)),)
	BUILD_GOPRIVATE:=GOPRIVATE=${BUILD_GOPRIVATE}
endif


.PHONY: print-global-variables

mcad-controller: init generate-code
	$(info Compiling controller)
	CGO_ENABLED=0 GOARCH=amd64 ${BUILD_GOPRIVATE} go build ${IMAGE_TAG_OPT} -o ${BIN_DIR}/mcad-controller ./cmd/kar-controllers/

print-global-variables:
	$(info "---")
	$(info "MAKE GLOBAL VARIABLES:")
	$(info "  "BIN_DIR="$(BIN_DIR)")
	$(info "  "BUILD_GOPRIVATE="$(BUILD_GOPRIVATE)")
	$(info "  "BUILD_TAG="$(BUILD_TAG)")
	$(info "  "GIT_BRANCH="$(GIT_BRANCH)")
	$(info "  "IMAGE_TAG="$(IMAGE_TAG)")
	$(info "  "IMAGE_TAG_OPT="$(IMAGE_TAG_OPT)")
	$(info "  "RELEASE_VER="$(RELEASE_VER)")
	$(info "---")

verify: generate-code
#	hack/verify-gofmt.sh
#	hack/verify-golint.sh
#	hack/verify-gencode.sh

init:
	mkdir -p ${BIN_DIR}

verify-tag-name: print-global-variables
	# Check for invalid tag name
	t=${IMAGE_TAG} && [ $${#t} -le 128 ] || { echo "Target name $$t has 128 or more chars"; false; }

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
	docker build --quiet --no-cache --tag mcad-controller:${IMAGE_TAG} -f ${CURRENT_DIR}/deployment/Dockerfile.both  ${CURRENT_DIR}/_output/bin

push-images: verify-tag-name
ifeq ($(strip $(dockerhub_repository)),)
	$(info No registry information provide.  To push images to a docker registry please set)
	$(info environment variables: dockerhub_repository, dockerhub_token, and dockerhub_id.  Environment)
	$(info variables do not need to be set for github Travis CICD.)
else
	$(info Log into dockerhub)
	docker login -u ${dockerhub_id} --password ${dockerhub_token}
	$(info Tag the latest image)
	docker tag mcad-controller:${IMAGE_TAG}  ${dockerhub_repository}/mcad-controller:${IMAGE_TAG}
	$(info Push the docker image to registry)
	docker push ${dockerhub_repository}/mcad-controller:${IMAGE_TAG}
endif

run-test:
	$(info Running unit tests...)
	hack/make-rules/test.sh $(WHAT) $(TESTS)

run-e2e: mcad-controller verify-tag-name
ifeq ($(strip $(dockerhub_repository)),)
	echo "Running e2e with MCAD local image: mcad-controller ${IMAGE_TAG} IfNotPresent ${BUILD_TAG}."
	hack/run-e2e-kind.sh mcad-controller ${IMAGE_TAG} IfNotPresent ${BUILD_TAG}
else
	echo "Running e2e with MCAD registry image image: ${dockerhub_repository}/mcad-controller ${IMAGE_TAG} Always ${BUILD_TAG}."
	hack/run-e2e-kind.sh ${dockerhub_repository}/mcad-controller ${IMAGE_TAG} Always ${BUILD_TAG}
endif

coverage:
#	KUBE_COVER=y hack/make-rules/test.sh $(WHAT) $(TESTS)

clean:
	rm -rf _output/
	rm -f mcad-controllers
