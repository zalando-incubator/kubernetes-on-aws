#!/bin/sh
set -e
MSG=/tmp/pr-message
git checkout alpha
git pull --rebase
git checkout beta
git pull --rebase
set +e
git branch -D alpha-to-beta
set -e
git checkout -b alpha-to-beta
git merge alpha
git push origin alpha-to-beta


echo "alpha-to-beta\n"> $MSG
git log --pretty=format:'%B' beta..alpha | egrep -iv '^$|^Merge |^Signed-off-by|^Dev to alpha|^alpha to beta' | uniq >> $MSG

if [ -x $(which hub) ]
then
  hub pull-request -b beta -h alpha-to-beta -F $MSG -l ready-to-test
else
  echo "============================
create a PR with base beta and head alpha-to-beta at https://github.com/zalando-incubator/kubernetes-on-aws
release notes:"
  cat $MSG
  echo "============================"
fi
