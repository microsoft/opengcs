# Archived

This project has been archived. Its contents have been merged into [Microsoft/hcsshim](https://github.com/microsoft/hcsshim) where development will continue. Some code organization has changed as part of the merge, please see below for details on where things have moved. Any issues regarding opengcs should be opened on the hcsshim repo.

Changes:
* Everything in /internal has been moved to [github.com/Microsoft/hcsshim/internal/guest](https://github.com/microsoft/hcsshim/tree/master/internal/guest) besides a few redundant packages that had equivalents in hcsshim. These include /internal/debug, /internal/oc and /internal/log. These are now located at github.com/Microsoft/hcsshim/internal/debug, github.com/Microsoft/hcsshim/internal/oc, and github.com/Microsoft/hcsshim/internal/log.
* /service/libs/commonutils has moved to github.com/Microsoft/hcsshim/internal/guest.
* Any non Go code lives at the root of github.com/Microsoft/hcsshim now, just as it did in this repository. This includes /vsockexec, /init, and /hack.
* The main gcs and gcstools binaries are now built and located at github.com/Microsoft/hcsshim/cmd/gcs and github.com/Microsoft/hcsshim/cmd/gcstools. 
* Makefile lives top level at github.com/Microsoft/hcsshim/Makefile and same commands will work to build everything.

# Open Guest Compute Service (opengcs) [![Build Status](https://github.com/microsoft/opengcs/actions/workflows/ci.yml/badge.svg)](https://github.com/microsoft/opengcs/actions/workflows/ci.yml)

Open Guest Compute Service is a Linux open source project to further the development of a production quality implementation of Linux Hyper-V containers on Windows (LCOW).  It's designed to run inside a custom Linux OS for supporting Linux container payload.

# LCOW v1 (deprecated)

The original version of `LCOW v1` was designed to run directly through `Docker` against the `HCS` (Host Compute Service) on Windows. This workflow is no longer supported by this repository however it has not been intentionally broken. If you would like to continue to use `LCOW v1` there is a branch `lcow_v1` that is the LKG branch previous to the removal of `LCOW v1` from the `master` branchline. All future efforts are focused on `LCOW v2`.

# LCOW v2

The primary difference between `LCOW v1` and `LCOW v2` is that `v1` was designed to hide the concept of the Utility VM. The caller created a _Linux container_ and operated on the container as if it was natively running on Windows. In the background a lightweight Utility VM was created that actually hosted the container but this was not visible and its resources not controllable via the caller. Although this works, it severely limited certain abilities such as the concept of Kubernetes pod or placing multiple LCOW containers in a single hypervisor boundary and set of resources.

Thus `LCOW v2` was created which has two primary differences.
- The Utility VM backing the Linux containers is a first class construct. Thus it can be managed in a lifetime separate from the actual containers running in it.
- The communication from host to guest is no longer done via the platform. This means that `LCOW v2` can iterate simply by improving its host/guest protocol with no need for taking Windows updates.

The focus of `LCOW v2` as a replacement of `LCOW v1` is through the coordination and work that has gone into [containerd/containerd](https://github.com/containerd/containerd) and its [Runtime V2](https://github.com/containerd/containerd/tree/master/runtime/v2) interface. To see our `containerd` hostside shim please look here [Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1](https://github.com/microsoft/hcsshim/tree/master/cmd/containerd-shim-runhcs-v1).

# Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.microsoft.com.

When you submit a pull request, a CLA-bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., label, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

We also ask that contributors [sign their commits](https://git-scm.com/docs/git-commit) using `git commit -s` or `git commit --signoff` to certify they either authored the work themselves or otherwise have permission to use it in this project. 

# Code of Conduct

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/). For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.
