BIN_DIR=_output/bin
RELEASE_VER=v0.2
CURRENT_DIR=$(shell pwd)

kar-controller: init
	CGO_ENABLED=0 GOARCH=amd64 go build -o ${BIN_DIR}/kar-controllers ./cmd/kar-controllers/

verify: generate-code
#	hack/verify-gofmt.sh
#	hack/verify-golint.sh
#	hack/verify-gencode.sh

init:
	mkdir -p ${BIN_DIR}

generate-code:
	$(info Compiling deepcopy-gen)
	go build -o ${BIN_DIR}/deepcopy-gen ./cmd/deepcopy-gen/
	$(info Generating deepcopy)
	${BIN_DIR}/deepcopy-gen -i ./pkg/apis/controller/v1alpha1/ -O zz_generated.deepcopy 

images: kube-batch
	cp ./_output/bin/kube-batch ./deployment/images/
	GOPATH=${ORIG_GOPATH} docker build ./deployment/images -t kubesigs/kube-batch:${RELEASE_VER}
	rm -f ./deployment/images/kube-batch

run-test:
#	hack/make-rules/test.sh $(WHAT) $(TESTS)

e2e: 
#	kube-controller
#	hack/run-e2e.sh

coverage:
#	KUBE_COVER=y hack/make-rules/test.sh $(WHAT) $(TESTS)

clean:
	rm -rf _output/
	rm -f kar-controllers 
