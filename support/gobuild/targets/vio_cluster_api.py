# Copyright 2017 VMware, Inc.  All rights reserved. -- VMware Confidential
"""
GoBuild target to build VIO Kubernetes Images.
"""

import os
import re

import helpers.env
import helpers.make
import helpers.target


class VIOClusterApi(helpers.target.Target, helpers.make.MakeHelper):
    """
    GoBuild target to build various OpenStack images. 
    """
    def GetBuildProductNames(self):
        return {'name': 'vio-cluster-api',
                'longname': 'cluster-api-provider-vsphere build used by VIO.Next'}

    def GetClusterRequirements(self):
        return ["linux-centos64-kernel-3.18.21"]

    def GetRepositories(self, hosttype):
        repos = [{"rcs": "git",
                  "src": "core-build/cluster-api-provider-vsphere.git;%(branch);",
                  "dst": "cluster-api-provider-vsphere"}]
        return repos

    def GetStorageInfo(self, hosttype):
        return [{"type": "source",
                 "src": "cluster-api-provider-vsphere"}]

    def GetCommands(self, hosttype):
        commands = []
        # Prepare build slave with docker
        prepare_slave_script = "%(buildroot)/cluster-api-provider-vsphere/support/utilities/install_docker.sh"
        commands.append({
            "desc": "Prepare build slave with docker and dependencies",
            "root": "%(buildroot)",
            "log": "gobuild_prepare_build_slave.log",
            "command": "sudo bash %(script)s %(root)s" % {
                "script": prepare_slave_script,
                "root":"%(buildroot)"
                },
            "env": self._GetEnvironment(hosttype)
        })

        target = 'all'
        flags = {}
        commands.append({
            'desc': 'Building cluster-api-provider-vsphere Images',
            'root': '%(buildroot)/cluster-api-provider-vsphere/support/',
            'log': '%s.log' % target,
            'command': self._Command(hosttype, target, **flags),
            'env': self._GetEnvironment(hosttype),
        })
        return commands

    def _GetEnvironment(self, hosttype):
        assert hosttype.startswith('linux')
        env = helpers.env.SafeEnvironment(hosttype)
        # Add mounted toolchain programs
        paths = map((lambda x: "/build/toolchain/lin64/" + x + "/bin"), [
            "python-2.7.8", "make-3.81", "coreutils-5.97", "tar-1.23"
        ])
        # Add cayman toolchain python with dependencies
        env['PATH'] = os.pathsep.join(paths + [
            "/build/toolchain//lin32/git-2.6.2/bin",
            "/build/toolchain/lin32/rsync-3.0.7/bin",
            env["PATH"]
        ])
        # Adding other paths
        env['PATH'] = os.pathsep.join(paths + [
            "/usr/bin", "/bin", "/sbin", "/usr/sbin",
            env["PATH"]
        ])
        env['PYTHONPATH'] = os.pathsep.join([
            '/build/toolchain/noarch/jinja2-2.5.5/lib/python2.7/site-packages',
            '/build/toolchain/lin64/pyyaml-3.10/lib/python2.7/site-packages',
            '/build/toolchain/noarch/argparse-1.1/lib/python2.6/site-packages'])
        return env
