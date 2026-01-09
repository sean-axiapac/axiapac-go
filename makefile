# 1. Identify the module (first word) and the tasks (remaining words)
MODULE := $(firstword $(MAKECMDGOALS))
TASKS  := $(wordlist 2, $(words $(MAKECMDGOALS)), $(MAKECMDGOALS))

# 2. Swallow the task names so root Make doesn't try to execute them as separate targets
$(eval $(TASKS):;@:)

.PHONY: oktedi axiapac-reply-email-handler sync-calendar no-task

oktedi:
	@$(MAKE) -f ./oktedi/makefile.mk $(TASKS)

axiapac-reply-email-handler:
	@$(MAKE) -f ./lambdas/axiapac-reply-email-handler/makefile.mk $(TASKS)

sync-calendar:
	@$(MAKE) -f ./lambdas/sync-calendar/makefile.mk $(TASKS)

no-task:
	@echo "❌ You must specify a module and a task (e.g. make oktedi build, make sync-calendar deploy)"
	@exit 1

.DEFAULT_GOAL := no-task
