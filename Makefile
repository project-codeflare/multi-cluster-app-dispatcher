BIN_DIR=_output/bin
RELEASE_VER=v1.16
CURRENT_DIR=$(shell pwd)
MCAD_REGISTRY=$(shell docker ps --filter name=mcad-registry | grep -v NAME)
LOCAL_HOST_NAME=localhost
dockerhub_repository=${LOCAL_HOST_NAME}:5000

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
	docker build --no-cache --tag mcad-controller:${RELEASE_VER} -f ${CURRENT_DIR}/deployment/Dockerfile.both  ${CURRENT_DIR}/_output/bin

remove_docker_registry:
ifeq ($(strip $(MCAD_REGISTRY)),)
	echo "MCAD Registry not running yet."
else
	docker rm -f mcad-registry
	echo "MCAD Registry Cleaned"
endif

push_images: remove_docker_registry
	$(info Create a private docker registry)
	docker run -d -p 5000:5000 --restart always --name mcad-registry registry:2
	#$(info Log into dockerhub)
	#echo ${dockerhub_token} | docker login -u ${dockerhub_id} --password-stdin
	$(info Tag the latest image)
	docker tag mcad-controller:${RELEASE_VER}  ${dockerhub_repository}/mcad-controller:${RELEASE_VER}
	$(info Push the docker image to registry)
	docker push ${dockerhub_repository}/mcad-controller:${RELEASE_VER}
	docker rmi ${dockerhub_repository}/mcad-controller:${RELEASE_VER}

run-test:
	$(info Running unit tests...)
	hack/make-rules/test.sh $(WHAT) $(TESTS)

run-e2e: mcad-controller
	hack/run-e2e-kind.sh ${dockerhub_repository}/mcad-controller ${RELEASE_VER}

coverage:
#	KUBE_COVER=y hack/make-rules/test.sh $(WHAT) $(TESTS)

clean:
	rm -rf _output/
	rm -f mcad-controllers
