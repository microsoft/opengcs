

# How to produce a custom Linux OS image

A LCOW custom Linx OS image was devided into two parts: a Linux kernel module and a set of user-mode componments. Both parts were highly customized for the purpose of supporting Linux Hyper-V container on Windows


# How to build custom kernel module

    In your 4.11 kernel source tree:

        Apply additional [4.11 patches](../kernelconfig/4.11/patches_readme.md/) to your 4.11 kernel source tree 
        Use the recommended [Kconfig](../kernelconfig/4.11/kconfig_for_4_11/) to build a 4.11 kernel that includes all LCOW necessary kernel componments.
        Build your kernel 


    Note:  The key delta between the upsteam default setting and above kconfig is in the area of ACPI/NIFT/NVDIMM/OverlyFS/9pFS/Vsock/HyerpV settings, which were set to be built-in instead of modules
         The Kconfig above is still a work in process in terms of eliminating unnecessary components from the kernel image. 

# How to construct user-mode components

    Under the / directory, the following directory structure is required:

    tmp proc bin dev run etc usr mnt sys    init root sbin lib64 lib      

    Here are the expected contents of each subdirectory /file
     
     1. Some of the subdirectories start with empty contents:  here they are: tmp proc dev run etc usr mnt sys 

     2. /init  ;  the init script file, which has the following contents

        #!/bin/sh
        export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

        # Configure everything before running GCS
        # Set up mounts
        mount -t proc proc /proc
        mount -t sysfs sysfs /sys
        mount -t devtmpfs udev /dev
        mount -t tmpfs tmpfs /run
        mount -t cgroup cgroup /sys/fs/cgroup

        mkdir /dev/mqueue
        mount -t mqueue mqueue /dev/mqueue
        mkdir /dev/pts
        mount -t devpts devpts /dev/pts
        mkdir /dev/shm
        mount -t tmpfs shm /dev/shm

        # Run gcs in the background
        cd /bin
        ./gcs  -loglevel=verbose -logfile=/tmp/gcslog.txt &
        cd -
        sh


     3./root : this is the home directory of the root account. At this moment, it contains a sandbox file with a prebuilt empty ext4 fs for supporting service vm operations
        
        /root/integration/prebuildSandbox.vhdx


     4./sbin : 
        /sbin/runc  

        Note:this is the "runc" binary for hosting the container execution environment. 
              It needs to be a version with the following release
              runc version 1.0.0-rc3
              commit: 992a5be178a62e026f4069f443c6164912adbf09
              spec: 1.0.0-rc5

        /sbin/udhcpc_config.script  ; see below for it contents
                             
                    #!/bin/sh
                    RESOLV_CONF="/etc/resolv.conf"

                    # dump the contents of the /etc/resolv.conf"
                    if [ -e $RESOLV_CONF ]; then
                       echo "initial contents of $RESOLV_CONF: used to configure a sysmtem Domain Name System resolver"
                       cat $RESOLV_CONF

                    else
                       echo "$RESOLV_CONF does not exist"
                    fi

                    [ -n "$1" ] || { echo "Error: should be called from udhcpc"; exit 1; }

                    echo "Parameter 1: $1"

                    NETMASK=""
                    [ -n "$subnet" ] && NETMASK="netmask $subnet"
                    BROADCAST="broadcast +"
                    [ -n "$broadcast" ] && BROADCAST="broadcast $broadcast"

                    case "$1" in
                            deconfig)
                                    echo $1
                                    echo "    Setting IP address 0.0.0.0 on $interface"
                                    ifconfig $interface 0.0.0.0
                                    ;;

                            renew|bound)
                                    echo $1
                                    echo "    Setting IP address $ip on $interface"
                                    ifconfig $interface $ip $NETMASK $BROADCAST
                                    echo "router = [$router]"
                                    if [ -n "$router" ] ; then
                                            echo "Deleting routers"
                                            while route del default gw 0.0.0.0 dev $interface ; do
                                                    :
                                            done

                                            metric=0
                                            for i in $router ; do
                                                    echo "Adding router $i"
                                                    route add default gw $i dev $interface metric $metric
                                                    : $(( metric += 1 ))
                                            done
                                    fi

                                    echo "Recreating $RESOLV_CONF"
                                    # If the file is a symlink somewhere (like /etc/resolv.conf
                                    # pointing to /run/resolv.conf), make sure things work.
                                    realconf=$(readlink -f "$RESOLV_CONF" 2>/dev/null || echo "$RESOLV_CONF")
                                    tmpfile="$realconf-$$"
                                    > "$tmpfile"
                                    echo "doamin=[$domain]"
                                    [ -n "$domain" ] && echo "search $domain" >> "$tmpfile"
                                    for i in $dns ; do
                                            echo " Adding DNS server $i"
                                            echo "nameserver $i" >> "$tmpfile"
                                    done
                                    mv "$tmpfile" "$realconf"
                                    ;;
                    esac
                    exit 0



     5./lib64 :
       /lib64/ld-linux-x86-64.so.2

     6./lib : 
       /lib/x86_64-linux-gnu
       /lib/x86_64-linux-gnu/libe2p.so.2
       /lib/x86_64-linux-gnu/libcom_err.so.2
       /lib/x86_64-linux-gnu/libc.so.6
       /lib/x86_64-linux-gnu/libdl.so.2
       /lib/x86_64-linux-gnu/libapparmor.so.1
       /lib/x86_64-linux-gnu/libseccomp.so.2
       /lib/x86_64-linux-gnu/libblkid.so.1
       /lib/x86_64-linux-gnu/libpthread.so.0
       /lib/x86_64-linux-gnu/libext2fs.so.2
       /lib/x86_64-linux-gnu/libuuid.so.1
       /lib/modules

      7./bin : key LCOW binaries stored in this directories
        
        // GCS binaries built from [here](./docs/gcsbuildinstructions.md/)
            /bin/gcs
            /bin/gcstools
            /bin/vhd2tar
            /bin/tar2vhd
            /bin/exportSandbox
            /bin/createSandbox

        // required binaires

             /bin/sh
             /bin/mkfs.ext4
             /bin/blockdev
             /bin/mkdir
             /bin/rmdir
             /bin/mount
             /bin/udhcpd
             /bin/ip
             /bin/iproute
             /bin/hostname

        // debugging tools

        [See complete user-mode file list](./kernelconfig/4.11/completeUsermodeFileLists.md/)

# Supported LCOW custom Linux OS packaing formats

    A LCOW custom Linux OS could be packaged into two different formats: 

    Kernel + Initrd: vmlinuz and initrd.img
    VHD: a VHDx file



