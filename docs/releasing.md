# Release process

## Manual

Push the release tag:

1. If this is a new minor release,
    1. Verify that the `metadata.yaml` file has an entry binding the CAPI contract
       to this new release version.
       If this is not yet done, create a new commit to add this entry.
    2. Create a new release branch and push
       to github, otherwise switch to it, for example `release-0.7`
2. Create an env var VERSION=v0.x.x Ensure the version is prefixed with a v
3. Tag the repository and push the tag `git tag -m $VERSION $VERSION`
    * If you have a GPG key for use with git use `git tag -s -m $VERSION $VERSION`
      to push a signed tag.
4. Push the tag using `git push upstream $VERSION`

Wait until:

* The release GitHub action created a GitHub draft release: [link](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/actions/workflows/release.yaml)
* The post-submit ProwJob pushed the image: [link](https://prow.k8s.io/?job=post-cluster-api-provider-vsphere-release)

Publish the release:

1. Review the release notes
2. Publish release. Use the pre-release option for release
   candidate versions of Cluster API Provider vSphere.
3. Email `kubernetes-sig-cluster-lifecycle@googlegroups.com` to
    announce the release
