# System: Disk Operations, Permissions, Services

# =============================================================================
section "System: Disk Operations"
# =============================================================================
expect_deny 'dd if=/dev/zero of=/dev/sda'              'dd to device'
expect_deny 'dd if=/dev/urandom of=/dev/sdb bs=1M'     'dd wipe device'
expect_deny 'fdisk /dev/sda'                           'fdisk edit'
expect_deny 'parted /dev/sda mklabel gpt'              'parted modify'
expect_deny 'mkfs.ext4 /dev/sda1'                      'mkfs format'
expect_deny 'wipefs -a /dev/sda'                       'wipefs device'
expect_deny 'mount --bind / /mnt'                      'mount bind root'
expect_deny 'umount -f /mnt'                           'umount force'
expect_deny 'losetup /dev/loop0 disk.img'              'losetup device'
expect_deny 'mdadm --stop /dev/md0'                    'mdadm stop'
expect_deny 'mdadm --remove /dev/md0 /dev/sda1'        'mdadm remove'
expect_deny 'mdadm --fail /dev/md0 /dev/sda1'          'mdadm fail'
expect_deny 'mdadm --zero-superblock /dev/sda1'        'mdadm zero-superblock'
expect_deny 'mdadm --create /dev/md0 --level=1 --raid-devices=2 /dev/sda1 /dev/sdb1' \
                                                       'mdadm create'
expect_deny 'mdadm --grow /dev/md0 --size=max'         'mdadm grow'
expect_deny 'btrfs subvolume delete /mnt/subvol'       'btrfs subvolume delete'
expect_deny 'btrfs device remove /dev/sdb /mnt/data'   'btrfs device remove'
expect_deny 'btrfs device add /dev/sdc /mnt/data'      'btrfs device add'
expect_deny 'btrfs balance start /mnt/data'            'btrfs balance'
expect_deny 'btrfs check --repair /dev/sda1'           'btrfs check --repair'
expect_deny 'btrfs rescue super-recover /dev/sda1'     'btrfs rescue'
expect_deny 'btrfs filesystem resize -5g /mnt/data'    'btrfs filesystem resize'
expect_deny 'dmsetup remove mydevice'                  'dmsetup remove'
expect_deny 'dmsetup remove_all'                       'dmsetup remove_all'
expect_deny 'dmsetup wipe_table mydevice'              'dmsetup wipe_table'
expect_deny 'dmsetup clear mydevice'                   'dmsetup clear'
expect_deny 'dmsetup load mydevice table.txt'          'dmsetup load'
expect_deny 'dmsetup create mydevice table.txt'        'dmsetup create'
expect_deny 'nbd-client -d /dev/nbd0'                  'nbd-client disconnect'
expect_deny 'nbd-client server 10809 /dev/nbd0'        'nbd-client connect'
expect_deny 'pvremove /dev/sda1'                       'pvremove'
expect_deny 'vgremove myvg'                            'vgremove'
expect_deny 'lvremove /dev/myvg/mylv'                  'lvremove'
expect_deny 'vgreduce myvg /dev/sda1'                  'vgreduce'
expect_deny 'lvreduce -L 10G /dev/myvg/mylv'           'lvreduce'
expect_deny 'lvresize -L -5G /dev/myvg/mylv'           'lvresize shrink'
expect_deny 'pvmove /dev/sda1 /dev/sdb1'               'pvmove'
expect_deny 'lvconvert --merge /dev/myvg/snap'         'lvconvert merge'

# Disk safe commands
expect_allow 'lsblk'                                   'lsblk'
expect_allow 'fdisk -l'                                'fdisk -l (list only)'
expect_allow 'df -h'                                   'df -h'
expect_allow 'blkid'                                   'blkid'
expect_allow 'dd if=/dev/zero of=output.img bs=1M count=100' \
                                                       'dd to file (not device)'
expect_allow 'parted /dev/sda print'                   'parted print (read-only)'
expect_allow 'mdadm --detail /dev/md0'                 'mdadm --detail (read-only)'
expect_allow 'mdadm --examine /dev/sda1'               'mdadm --examine (read-only)'

# =============================================================================
section "System: Permissions"
# =============================================================================
expect_deny 'chmod 777 /etc/passwd'                    'chmod 777 sensitive file'
expect_deny 'chmod -R 777 /'                           'chmod -R 777 root'
expect_deny 'chown -R root:root /'                     'chown -R root on root'
expect_deny 'chmod u+s /usr/bin/myapp'                 'chmod setuid'
expect_deny 'chmod g+s /usr/bin/myapp'                 'chmod setgid'
expect_deny 'chown root: /usr/bin/myapp'               'chown to root'
expect_deny 'setfacl -R -m u:nobody:rwx /etc'          'setfacl recursive on /etc'

# =============================================================================
section "System: Services"
# =============================================================================
expect_deny 'systemctl stop sshd'                      'systemctl stop sshd'
expect_deny 'systemctl stop docker'                    'systemctl stop docker'
expect_deny 'systemctl stop containerd'                'systemctl stop containerd'
expect_deny 'systemctl stop firewalld'                 'systemctl stop firewalld'
expect_deny 'service sshd stop'                        'service sshd stop'
expect_deny 'systemctl isolate rescue.target'          'systemctl isolate'
expect_deny 'systemctl poweroff'                       'systemctl poweroff'
expect_deny 'systemctl reboot'                         'systemctl reboot'
expect_deny 'shutdown -h now'                          'shutdown -h now'
expect_deny 'reboot'                                   'reboot'
expect_deny 'init 0'                                   'init 0'
expect_deny 'init 6'                                   'init 6'
