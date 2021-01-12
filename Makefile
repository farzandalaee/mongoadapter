# Makefile for Mongo Adapter
include .env
export
NAME=gitlab.faza.io/go-framework/mongoadapter
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOMODTIDY=$(GOCMD) mod tidy
GOMODDOWNLOAD=$(GOCMD) mod download

.PHONY: test-full
test-full:
	-docker container rm -f mongo
	$(GOMODTIDY) && $(GOMODDOWNLOAD)
	echo "Starting a container mapped to port ${GOTEST_MONGO_PORT}"
	docker run -d --rm --name mongo -p ${GOTEST_MONGO_PORT}:27017 mongo:4.0.10
	-docker container rm -f mongo2
	docker run -d --rm --name mongo2 -p ${GOTEST_MONGO2_PORT}:27017 mongo:4.0.10
	@printf "\nRunning tests..."
	@printf "\n================="
	@printf "\nTEST STARTS"
	@printf "\n=================\n"
	GOPRIVATE=*.faza.io $(GOTEST) $(NAME)
	@printf "\n================="
	@printf "\nTEST ENDS"
	@printf "\n=================\n"

.PHONY: only-docker
only-docker:
	-docker container rm -f mongo
	-docker run -d --rm --name mongo -p ${GOTEST_MONGO_PORT}:27017 mongo:4.0.10
	-docker container rm -f mongo2
	-docker run -d --rm --name mongo2 -p ${GOTEST_MONGO2_PORT}:27017 mongo:4.0.10

.PHONY: test-simple
test-simple:
	GOPRIVATE=*.faza.io $(GOMODTIDY) && $(GOMODDOWNLOAD)
	GOPRIVATE=*.faza.io $(GOTEST) $(NAME)

.PHONY: test-code
test-code:
	GOPRIVATE=*.faza.io $(GOTEST) $(NAME)