#!/bin/sh
set -e
MSG=/tmp/pr-message
git checkout dev
git pull --rebase
git checkout alpha
git pull --rebase
set +e
git branch -D dev-to-alpha
set -e
git checkout -b dev-to-alpha
git merge dev
git push origin dev-to-alpha

echo "dev-to-alpha\n"> $MSG
git log --pretty=format:'%B' alpha..dev | egrep -iv '^$|^Merge |^Signed-off-by|^Dev to alpha' | uniq >> $MSG

if [ -x $(which hub) ]
then
  hub pull-request -b alpha -h dev-to-alpha -F $MSG -l ready-to-test
else
  echo "============================
create a PR with base alpha and head dev-to-alpha at https://github.com/zalando-incubator/kubernetes-on-aws
release notes:"
  cat $MSG
  echo "============================"
fi
