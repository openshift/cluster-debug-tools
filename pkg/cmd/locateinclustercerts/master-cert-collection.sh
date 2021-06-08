#!/bin/bash

mkdir -p /must-gather/certs
echo "Certificate (not key) Collection
1.0.0.0" > /must-gather/version

shopt -s globstar
for i in /host/etc/kubernetes/**/*.crt; do
  cp --parents "$i" /must-gather/certs
  # store the original listing information so we can see users, permissions, and selinux labels
  ls -alZR "$i" > "/must-gather/certs/$i.listing.txt"
  key=${i%.*}.key
  ls -alZR "$key" > "/must-gather/certs/$i.keylisting.txt"
done

#for i in /host/var/lib/kubelet/**/*.crt; do
#  cp --parents "$i" /must-gather/certs
#  # store the original listing information so we can see users, permissions, and selinux labels
#  ls -alZR "$i" > "/must-gather/certs/$i.listing.txt"
#done

sleep 30