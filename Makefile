all clean nuke:
	$(MAKE) -C pkg $@
	$(MAKE) -C cmd $@

testpackage:
	$(MAKE) -C pkg $@
