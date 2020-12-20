BIN_DIR=_output/bin
CAT_CMD=$(if $(filter $(OS),Windows_NT),type,cat)
VERSION_FILE=./CONTROLLER_VERSION
RELEASE_VER=v$(shell $(CAT_CMD) $(VERSION_FILE))
CURRENT_DIR=$(shell pwd)
GIT_BRANCH=$(git symbolic-ref --short HEAD 2>&1 | grep -v fatal)

mcad-controller: init generate-code
	$(info Compiling controller)
	CGO_ENABLED=0 GOARCH=amd64 go build -o ${BIN_DIR}/mcad-controller ./cmd/kar-controllers/

verify: generate-code
#	hack/verify-gofmt.sh
#	hack/verify-golint.sh
#	hack/verify-gencode.sh

init:
	mkdir -p ${BIN_DIR}

generate-code:
	$(info Compiling deepcopy-gen...)
	go build -o ${BIN_DIR}/deepcopy-gen ./cmd/deepcopy-gen/
	$(info Generating deepcopy...)
	${BIN_DIR}/deepcopy-gen -i ./pkg/apis/controller/v1alpha1/ -O zz_generated.deepcopy 

images:
	$(info List executable directory)
	ls -l ${CURRENT_DIR}/_output/bin
	$(info Build the docker image)
	docker build --quiet --no-cache --tag mcad-controller:${RELEASE_VER} -f ${CURRENT_DIR}/deployment/Dockerfile.both  ${CURRENT_DIR}/_output/bin

push-images:
ifeq ($(strip $(dockerhub_repository)),)
	$(info No registry information provide.  To push images to a docker registry please set)
	$(info environment variables: dockerhub_repository, dockerhub_token, and dockerhub_id.  Environment)
	$(info variables do not need to be set for github Travis CICD.)
else
	$(info Log into dockerhub)
	docker login -u ${dockerhub_id} --password ${dockerhub_token}
	$(info Tag the latest image)
	docker tag mcad-controller:${RELEASE_VER}  ${dockerhub_repository}/mcad-controller:${RELEASE_VER}
	$(info Push the docker image to registry)
	docker push ${dockerhub_repository}/mcad-controller:${RELEASE_VER}
endif

run-test:
	$(info Running unit tests...)
	hack/make-rules/test.sh $(WHAT) $(TESTS)

run-e2e: mcad-controller
ifeq ($(strip $(dockerhub_repository)),)
	echo "Running e2e with MCAD local image: mcad-controller ${RELEASE_VER} IfNotPresent."
	hack/run-e2e-kind.sh mcad-controller ${RELEASE_VER} IfNotPresent
else
	echo "Running e2e with MCAD registry image image: ${dockerhub_repository}/mcad-controller ${RELEASE_VER}."
	hack/run-e2e-kind.sh ${dockerhub_repository}/mcad-controller ${RELEASE_VER}
endif

coverage:
#	KUBE_COVER=y hack/make-rules/test.sh $(WHAT) $(TESTS)

clean:
	rm -rf _output/
	rm -f mcad-controllers
