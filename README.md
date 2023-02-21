## Installation

The following dependencies are required:
- golang $\geq$ 1.7.4
- vdeplug4
- Docker

The plugin is installed as a out-of-process daemon.

clone repo

    $ git clone https://github.com/mapokko/vde_plug_docker_v2.git
    $ cd /vde_plug_docker_v2

install go dependencies

    $ go mod tidy

build

    $ go build .

install daemon services

    $ sudo make install

A VM image with all the required dependencies can be downloaded [Here]([www.google.com](https://liveunibo-my.sharepoint.com/:u:/g/personal/fabio_mirza_studio_unibo_it/EZEcTcTMJPFHu3DKgT2fSPoB0KLjc1P5GedZMcK6kyhA3w?e=k9fcTJ)). The VM requires qemu-system-x86 to run.

Start the VM and login to "user" with password "virtualsquare"

    $ qemu-system-x86_64 -enable-kvm -smp $(nproc) -m 2G -monitor stdio -cpu host -netdev type=user,id=net,hostfwd=tcp::2222-:22 -device virtio-net-pci,netdev=net -drive file=$(echo debian-sid-v2-amd64-daily-20230221-1298.qcow2)

Create a network with "vde" as network driver

    $ sudo docker network create -d vde -o sock=vxvde://239.1.2.3 -o if=vd --subnet 10.10.0.1/24 vdenet

create a new contianer connected to the network

    $ sudo docker run -it --net vdenet --ip 10.10.0.2 debian &

now we can check the datastore to verify the network and the endpoint

    $ cat /etc/docker/vde_plug_docker.json

Create a TAP device on the host machine

    $ sudo ip tuntap add dev tap0 mode tap
    $ sudo ip addr add 10.10.0.5/24 dev tap0
    $ sudo ip link set tap0 up

verify connection with container throught VDE network

    $ ping -I 10.10.0.5 10.10.0.2