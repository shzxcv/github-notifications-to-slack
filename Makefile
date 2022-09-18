IMAGENAME=github-notifications-to-slack

.PHONY: debug
debug:
	docker build . -t $(IMAGENAME)
	docker run $(IMAGENAME)

.PHONY: prune
prune:
	docker rmi -f $$(docker images -f "dangling=true" -q)
