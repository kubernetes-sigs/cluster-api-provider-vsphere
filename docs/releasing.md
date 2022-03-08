# Release process

## Manual

1. Login using `gcloud auth login`. You should see
   `cluster-api-provider-vsphere` in your list of available projects.
2. Make sure your repo is clean by git's standards
3. If this is a new minor release,
   1. Verify that the `metadata.yaml` file has an entry binding the CAPI contract
      to this new release version.
      If this is not yet done, create a new commit to add this entry.
   2. Create a new release branch and push
      to github, otherwise switch to it, for example `release-0.7`
4. Create an env var VERSION=v0.x.x Ensure the version is prefixed with a v
5. Tag the repository and push the tag `git tag -m $VERSION $VERSION`
   * If you have a GPG key for use with git use `git tag -s -m $VERSION $VERSION`
   to push a signed tag.
6. Push the tag using `git push upstream $VERSION`
7. Create a draft release in github and associate it with the tag that
   was just created.
8. Checkout the tag you've just created and make sure git is in a clean
   state
9. Run `hack/release.sh -l` to generate the docker image. Use `-l` if
   the `latest` tag should also be applied to the Docker image. Make a
   note of the docker image, which should be of the format
   manager:${VERSION}. Ignore the manifest:${VERSION}.
10. Run `make release-manifests VERSION=$VERSION` to generate release
    manifests in the `/out` directory, and attach them to the draft
    release.
11. Push the created docker image, as well as the latest tag, if required.
12. Finalise the release notes
13. Publish release. Use the pre-release option for release
    candidate versions of Cluster API Provider vSphere.
14. Email `kubernetes-sig-cluster-lifecycle@googlegroups.com` to
    announce the release
