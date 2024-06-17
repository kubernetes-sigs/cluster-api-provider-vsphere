# janitor

The janitor is a tool for CI to cleanup objects leftover from failed or killed prowjobs.
It can be run regularly as prowjob.

It retrieves vSphere projects from Boskos and then deletes VMs and resource pools accordingly.
Additionally it will delete cluster modules which do not refer any virtual machine.
