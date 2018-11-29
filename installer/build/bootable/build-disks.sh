#!/bin/bash
# Copyright 2018 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# this file generates disks and grub configs for the vic product appliance
set -e -o pipefail +h && [ -n "$DEBUG" ] && set -x
DIR=$(dirname "$(readlink -f "$0")")
. "${DIR}/log.sh"

function setup_grub() {
  disk=$1
  boot_device="${1}p1"
  root_device="${1}p2"
  root=$2

  log3 "install grub to ${brprpl}${root}/boot${reset} on ${brprpl}${disk}${reset}" 

  mkdir -p "${root}/boot/grub2"
  ln -sfv grub2 "${root}/boot/grub"
  rm -rf "${root}/boot/grub2/fonts"
  curl -L"#" -o "${root}/boot/grub2/ascii.pf2" https://storage.googleapis.com/cluster-api-provider-vsphere-build/boot/ascii.pf2
  mkdir -p "${root}/boot/grub2/themes/photon"
  curl -L"#" -o "${root}/boot/grub2/themes/photon/photon.png" https://storage.googleapis.com/cluster-api-provider-vsphere-build/boot/splash.png
  curl -L"#" -o "${root}/boot/grub2/themes/photon/terminal_c.tga" https://storage.googleapis.com/cluster-api-provider-vsphere-build/boot/terminal_c.tga
  curl -L"#" -o "${root}/boot/grub2/themes/photon/terminal_e.tga" https://storage.googleapis.com/cluster-api-provider-vsphere-build/boot/terminal_e.tga
  curl -L"#" -o "${root}/boot/grub2/themes/photon/terminal_n.tga" https://storage.googleapis.com/cluster-api-provider-vsphere-build/boot/terminal_n.tga
  curl -L"#" -o "${root}/boot/grub2/themes/photon/terminal_ne.tga" https://storage.googleapis.com/cluster-api-provider-vsphere-build/boot/terminal_ne.tga
  curl -L"#" -o "${root}/boot/grub2/themes/photon/terminal_nw.tga" https://storage.googleapis.com/cluster-api-provider-vsphere-build/boot/terminal_nw.tga
  curl -L"#" -o "${root}/boot/grub2/themes/photon/terminal_s.tga" https://storage.googleapis.com/cluster-api-provider-vsphere-build/boot/terminal_s.tga
  curl -L"#" -o "${root}/boot/grub2/themes/photon/terminal_se.tga" https://storage.googleapis.com/cluster-api-provider-vsphere-build/boot/terminal_se.tga
  curl -L"#" -o "${root}/boot/grub2/themes/photon/terminal_sw.tga" https://storage.googleapis.com/cluster-api-provider-vsphere-build/boot/terminal_sw.tga
  curl -L"#" -o "${root}/boot/grub2/themes/photon/terminal_w.tga" https://storage.googleapis.com/cluster-api-provider-vsphere-build/boot/terminal_w.tga
  curl -L"#" -o "${root}/boot/grub2/themes/photon/theme.txt" https://storage.googleapis.com/cluster-api-provider-vsphere-build/boot/theme.txt

# EFI Install
  mkdir -p "$root/boot/efi"
  mount -t vfat "${boot_device}" "$root/boot/efi"
  grub2-install --target=x86_64-efi --efi-directory="${root}"/boot/efi --bootloader-id=Boot --root-directory="${root}" --recheck
  rm "${root}/boot/efi/EFI/Boot/grubx64.efi"
  
  curl -L"#" -o "${root}/boot/efi/EFI/Boot/bootx64.efi" https://storage.googleapis.com/cluster-api-provider-vsphere-build/EFI/BOOT/bootx64.efi
  curl -L"#" -o "${root}/boot/efi/EFI/Boot/grubx64.efi" https://storage.googleapis.com/cluster-api-provider-vsphere-build/EFI/BOOT/grubx64.efi
  mkdir -p "${root}/boot/efi/boot/grub2"

  log3 "configure grub"

  UUID_VAL=$(blkid -s UUID -o value "${root_device}")
  PARTUUID_VAL=$(blkid -s PARTUUID -o value "${root_device}")
  BOOT_UUID=$(blkid -s UUID -o value "${boot_device}")
  BOOT_DIRECTORY=/boot/

  # echo "configfile (hd0,gpt1)/boot/grub2/grub.cfg" > "${root}"/boot/efi/boot/grub2/grub.cfg
  echo "search -n -u ${UUID_VAL} -s" > "${root}"/boot/efi/boot/grub2/grub.cfg
  echo "configfile /boot/grub2/grub.cfg" >> "${root}"/boot/efi/boot/grub2/grub.cfg

  umount "${root}/boot/efi"

  # linux-esx tries to mount rootfs even before nvme got initialized.
  # rootwait fixes this issue
  EXTRA_PARAMS=""
  if [[ "$1" == *"nvme"* ]]; then
      EXTRA_PARAMS=rootwait
  fi

  cp "${DIR}/boot/grub.cfg" "${root}/boot/grub2/grub.cfg"

  sed -i "s/PARTUUID_PLACEHOLDER/$PARTUUID_VAL/" "${root}/boot/grub2/grub.cfg"
  sed -i "s/UUID_PLACEHOLDER/$UUID_VAL/" "${root}/boot/grub2/grub.cfg"
  
}

function create_disk() {
  local img="$1"
  local disk_size="$2"
  local mp="$3"
  local boot="${4:-}"
  cd "${PACKAGE}"

  losetup -f || ( echo "Cannot setup loop devices" && exit 1 )

  log3 "allocating raw image of ${brprpl}${disk_size}${reset}"
  fallocate -l "$disk_size" -o 1024 "$img"

  log3 "wiping existing filesystems"
  sgdisk -Z -og "$img"

  part_num=1

  if [[ -n $boot ]]; then
    log3 "creating EFI System partition"
    sgdisk -n $part_num:4096:413695 -c $part_num:"EFI System Partition" -t $part_num:ef00 "$img"
    part_num=$((part_num+1))
  fi

  log3 "creating linux partition"
  ENDSECTOR=$(sgdisk -E "$img")
  sgdisk -n $part_num:823296:"$ENDSECTOR" -c $part_num:"Linux system" -t $part_num:8300 "$img"

  log3 "reloading loop devices"
  disk=$(losetup --show -f -P "$img")

  if [[ -n $boot ]]; then
    part_num=1
    log3 "formatting EFI System partition"
    mkfs.fat -F32 "${disk}p$part_num"
    part_num=$((part_num+1))
  fi

  log3 "formatting linux partition"
  mkfs.ext4 -F "${disk}p$part_num" 

  log3 "mounting partition ${brprpl}${disk}p$part_num${reset} at ${brprpl}${mp}${reset}"
  mkdir -p "$mp"
  mount "${disk}p$part_num" "$mp"

  if [[ -n $boot ]]; then
    log3 "setup grup on boot disk"
    setup_grub "$disk" "$mp"
  fi
  
}

function convert() {
  local dev=$1
  local mount=$2
  local raw=$3
  local vmdk=$4
  cd "${PACKAGE}"
  log3 "unmount ${brprpl}${mount}${reset}"
  if mountpoint "${mount}" >/dev/null 2>&1; then
    umount -R "${mount}/" >/dev/null 2>&1
  fi

  log3 "release loopback device ${brprpl}${dev}${reset}"
  losetup -d "$dev"

  log3 "converting raw image ${brprpl}${raw}${reset} into ${brprpl}${vmdk}${reset}"
  qemu-img convert -f raw -O vmdk -o 'compat6,adapter_type=lsilogic,subformat=streamOptimized' "$raw" "$vmdk"
  rm "$raw"
}

function usage() {
  echo "Usage: $0 -p package-location -a [create|export] -i NAME -s SIZE -r ROOT [-i NAME -s SIZE -r ROOT]..."
  echo "  -p package-location   the working directory to use"
  echo "  -a action             the action to perform (create or export)"
  echo "  -i name               the name of an image"
  echo "  -s size               the size of an image"
  echo "  -r root               the mount point for the root of an image, relative to the package-location"
  echo "Example: $0 -p /tmp -a create -i vic-disk1.vmdk -s 6GiB -r mnt/root -i vic-disk2.vmdk -s 2GiB -r mnt/data"
  echo "Example: $0 -p /tmp -a create -i vic-disk1.vmdk -i vic-disk2.vmdk -s 6GiB -s 2GiB -r mnt/root -r mnt/data"
  exit 1
}

while getopts "p:a:i:s:r:" flag
do
    case $flag in

        p)
            # Required. Package name
            PACKAGE="$OPTARG"
            ;;

        a)
            # Required. Action: create or export
            ACTION="$OPTARG"
            ;;

        i)
            # Required, multi. Ordered list of image names
            IMAGES+=("$OPTARG")
            ;;

        s)
            # Required, multi. Ordered list of image sizes
            IMAGESIZES+=("$OPTARG")
            ;;

        r)
            # Required, multi. Ordered list of image roots
            IMAGEROOTS+=("$OPTARG")
            ;;

        *)
            usage
            ;;
    esac
done
shift $((OPTIND-1))

# check there were no extra args, the required ones are set, and an equal number of each disk argument were supplied
if [[ -n "$*" || -z "${PACKAGE}" || -z "${ACTION}" || ${#IMAGES[@]} -eq 0 || ${#IMAGES[@]} -ne ${#IMAGESIZES[@]} || ${#IMAGES[@]} -ne ${#IMAGEROOTS[@]} ]]; then
    usage
fi

if [ "${ACTION}" == "create" ]; then
  log1 "create disk images"
  for i in "${!IMAGES[@]}"; do
    BOOT=""
    [ "$i" == "0" ] && BOOT="1"
    log2 "creating ${IMAGES[$i]}.img"
    create_disk "${IMAGES[$i]}.img" "${IMAGESIZES[$i]}" "${PACKAGE}/${IMAGEROOTS[$i]}" $BOOT
  done

elif [ "${ACTION}" == "export" ]; then
  log1 "export images to VMDKs"
  for i in "${!IMAGES[@]}"; do
    log2 "exporting ${IMAGES[$i]}.img to ${IMAGES[$i]}.vmdk"
    echo "export ${PACKAGE}/${IMAGES[$i]}"
    DEV=$(losetup -l -O NAME,BACK-FILE -a | tail -n +2 | grep "${PACKAGE}/${IMAGES[$i]}" | awk '{print $1}')
    convert "${DEV}" "${PACKAGE}/${IMAGEROOTS[$i]}" "${IMAGES[$i]}.img" "${IMAGES[$i]}.vmdk"
  done

  log2 "VMDK Sizes"
  log2 "$(du -h ./*.vmdk)"

else
  usage

fi
