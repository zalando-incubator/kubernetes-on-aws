#!/bin/bash
# Helper script to update the authorized_keys section in the user data

ADMIN_USERS='''
ahartmann
hjacobs
jmussler
mkerk
mlarsen
mlinkhorst
rdifazio
sszuecs
ytussupbekov
'''

for file in userdata-*.yaml; do
    echo "Updating ${file}.."
    new_file=${file}.tmp
    echo '#cloud-config' > $new_file
    echo 'ssh_authorized_keys:' >> $new_file
    for uid in $ADMIN_USERS; do
        key=$(/usr/bin/curl -s https://even.stups.zalan.do/public-keys/$uid/sshkey.pub)
        echo "  - '$key'" >> $new_file
    done
    cat $file | awk '/^coreos:/ { p=1 } p==1 { print }' >> $new_file
    mv $new_file $file
done
