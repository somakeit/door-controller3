GOVARS=GOOS=linux GOARCH=arm GOARM=5
GC=go

.PHONY: doord
doord:
	$(GOVARS) $(GC) build -o doord cmd/doord/main.go

.PHONY: test
test:
	$(GOVARS) $(GC) build -o test cmd/test/main.go

.PHONY: all
all: doord test

.PHONY: clean
clean:
	rm -f doord test
