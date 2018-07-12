# Introduction

These instructions are intended to make it easier for developers to create new
Cluster API provider repositories. Arguably, it may be easier to just use
`apiserver-builder` directly. In any case, there is no intent to maintain these
instructions. Better automation or more sophisticated code sharing should be
preferred over this duck tape and bubble gum.  ¯\_(ツ)_/¯

# Quickstart

- The name of provider repos [should](
https://github.com/kubernetes-sigs/cluster-api/issues/383) be of the form
`cluster-api-provider- $(cloud)`. For example, `cluster-api-provider-openshift`
is the name of the repo which implements the Cluster API provider for OpenShift.

- [Create](https://help.github.com/articles/creating-a-new-repository/) a new
empty GitHub repo under your org using the GitHub GUI, for example
https://github.com/samsung-cnct/cluster-api-provider-ssh.

- [Duplicate](https://help.github.com/articles/duplicating-a-repository/)
this repo (https://github.com/samsung-cnct/cluster-api-provider-skeleton) and
push it to the `cluster-api-provider-ssh` repo you created in the previous
step. Note the arguments to clone and push.

```
git clone --bare https://github.com/samsung-cnct/cluster-api-provider-skeleton.git
cd cluster-api-provider-skeleton.git
git push --mirror https://github.com/samsung-cnct/cluster-api-provider-ssh.git
cd ..
rm -rf cluster-api-provider-skeleton.git
```

- Customize the new repository. A simple search and replace may suffice for
some name changes, e.g. on OS X, something like this might work:

```
find . -type f -path ./.git -prune -o -print -exec sed -i '' s/skeleton/ssh/ {} +
```

The following directorie must be moved:

```
git mv cloud/skeleton/ cloud/ssh/
git mv clusterctl/examples/skeleton/ clusterctl/examples/ssh/
```

For other changes, like the README.md, OWNERS_ALIASES, etc., you'll have to
think more.
