# janitor

The janitor is a tool for CI to cleanup objects leftover from failed or killed prowjobs.
It can be run regularly as prowjob.

It tries to delete:

* vSphere: virtual machines in the configured folders which exist longer than the configured `--max-age` flag.
* vSphere: cluster modules which do not refer any virtual machine
* IPAM: IPAddressClaims which exist longer than the configured `--max-age` flag
