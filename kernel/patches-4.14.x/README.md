# How to build 4.14.x based custom Linux kernel for LCOW

You can download the Linux 4.14 source code from kernel.org

Once you get the _4.14 kernel_, apply all the following patches 

## 1. Patch for "nvdimm: Lower minimum PMEM size"

The [patch file](./patches-4.11.x/0002-NVDIMM-reducded-ND_MIN_NAMESPACE_SIZE-from-4MB-to-4K.patch could be find in this directory.   

You should be in the Linux kernel source directory before applying the patch with the following command

```
patch -p1 < /path/to/kernel/patches-4.14.x/0002-*
```


## 2. Patch set for the Hyper-V vsock support

These patches enables the **Hyper-V vsock transport** feature,
this instructions is to get them from a developer repository and
assuming you have a _Linux GIT repository_  already

```
git config --global user.name "yourname"
git config --global user.email youremailaddress 
 
git remote add -f dexuan-github https://github.com/dcui/linux.git
 
git cherry-pick d4ed5cfe7acffb1df92d11cb51f30faadbba171b
git cherry-pick f77d3c692e3d5182286f02dbe683463802afc77a
git cherry-pick f8dd01899c1cdab7600d7df50dbd25dbcf891072

```

Another way to get the patches is to download them from the following list and
apply them in the same order:

1.  https://github.com/dcui/linux/commit/d4ed5cfe7acffb1df92d11cb51f30faadbba171b.patch
2.  https://github.com/dcui/linux/commit/f77d3c692e3d5182286f02dbe683463802afc77a.patch
3.  https://github.com/dcui/linux/commit/f8dd01899c1cdab7600d7df50dbd25dbcf891072.patch

