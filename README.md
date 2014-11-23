# THE PLAN

* each container has its own working directory

* stream in/out are relative to this directory

* running processes are relative to this directory

* containers are not isolated beyond that

* don't use this in a multitentant environment
    * seriously

* when a container is stopped or destroyed, all of its processes are killed

* does not implement snapshotting (yet?)

* limits and most other Garden calls are not supported (e.g. networking
  configuration)
